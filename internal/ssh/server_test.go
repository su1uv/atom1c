package ssh

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/database"
	gossh "golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"
)

func TestSSHAuthenticationAndInteractiveSession(t *testing.T) {
	home := t.TempDir()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keysPath := filepath.Join(home, "authorized_keys")
	if err := os.WriteFile(keysPath, gossh.MarshalAuthorizedKey(signer.PublicKey()), 0o600); err != nil {
		t.Fatal(err)
	}
	config := Config{
		Address:        "127.0.0.1:23234",
		AuthorizedKeys: keysPath,
		HostKey:        filepath.Join(home, "host_key"),
		Username:       "owner",
	}
	db := openSSHTestDB(t)
	defer db.Close()
	state := &internal.State{Db: database.New(db), SQLDB: db}
	server, err := newServer(config, state)
	if err != nil {
		t.Fatalf("newServer() = %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.serve(listener) }()
	defer func() {
		_ = server.close()
		select {
		case <-serveDone:
		case <-time.After(3 * time.Second):
			t.Error("SSH server did not stop")
		}
	}()

	clientConfig := func(username string) *gossh.ClientConfig {
		return &gossh.ClientConfig{
			User:            username,
			Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
			Timeout:         3 * time.Second,
		}
	}
	if client, err := gossh.Dial("tcp", listener.Addr().String(), clientConfig("intruder")); err == nil {
		_ = client.Close()
		t.Fatal("SSH accepted a different username with the owner's key")
	}
	client, err := gossh.Dial("tcp", listener.Addr().String(), clientConfig("owner"))
	if err != nil {
		t.Fatalf("authorized public-key login: %v", err)
	}
	defer client.Close()

	subsystemSession, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := subsystemSession.RequestSubsystem("sftp"); err == nil {
		_ = subsystemSession.Close()
		t.Fatal("SSH accepted the SFTP subsystem")
	}
	_ = subsystemSession.Close()
	forwardPayload := gossh.Marshal(struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}{Host: "127.0.0.1", Port: 80, OriginHost: "127.0.0.1", OriginPort: 10000})
	if _, _, err := client.OpenChannel("direct-tcpip", forwardPayload); err == nil {
		t.Fatal("SSH accepted a local port-forward channel")
	}
	forwarded, _, err := client.SendRequest("tcpip-forward", true, gossh.Marshal(struct {
		Host string
		Port uint32
	}{Host: "127.0.0.1", Port: 0}))
	if err != nil {
		t.Fatalf("request reverse port forwarding: %v", err)
	}
	if forwarded {
		t.Fatal("SSH accepted reverse port forwarding")
	}

	withoutPTY, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := withoutPTY.Shell(); err == nil {
		_ = withoutPTY.Close()
		t.Fatal("SSH accepted an interactive shell without a PTY")
	}
	_ = withoutPTY.Close()

	commandSession, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := commandSession.Run("true"); err == nil {
		_ = commandSession.Close()
		t.Fatal("SSH accepted an exec command")
	}
	_ = commandSession.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
		t.Fatalf("request PTY: %v", err)
	}
	if err := session.Shell(); err != nil {
		t.Fatalf("start interactive shell: %v", err)
	}
	if err := session.WindowChange(30, 100); err != nil {
		t.Fatalf("resize interactive shell: %v", err)
	}
	if _, err := io.WriteString(stdin, "aSSH feed\thttps://example.test/feed.atom\t\r"); err != nil {
		t.Fatalf("send add-feed interaction: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM feeds WHERE name = ? AND url = ?`, "SSH feed", "https://example.test/feed.atom").Scan(&count); err != nil {
			t.Fatalf("check feed saved over SSH: %v", err)
		}
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("interactive SSH session did not save the feed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	firstWait := make(chan error, 1)
	go func() { firstWait <- session.Wait() }()
	secondSession, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secondOutput, err := secondSession.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, secondOutput) }()
	secondInput, err := secondSession.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := secondSession.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
		t.Fatalf("request second session PTY: %v", err)
	}
	if err := secondSession.Shell(); err != nil {
		t.Fatalf("start second interactive shell: %v", err)
	}
	if _, err := io.WriteString(secondInput, "q"); err != nil {
		t.Fatalf("quit second SSH session: %v", err)
	}
	secondWait := make(chan error, 1)
	go func() { secondWait <- secondSession.Wait() }()
	select {
	case err := <-secondWait:
		if err != nil {
			t.Fatalf("second interactive TUI did not exit cleanly: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("q did not close the second SSH session")
	}
	select {
	case err := <-firstWait:
		t.Fatalf("quitting second session also ended first session: %v", err)
	default:
	}
	if _, err := io.WriteString(stdin, "q"); err != nil {
		t.Fatalf("quit first SSH session: %v", err)
	}
	select {
	case err := <-firstWait:
		if err != nil {
			t.Fatalf("first interactive TUI did not exit cleanly: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("q did not close first interactive SSH session")
	}
}

func openSSHTestDB(t *testing.T) *sql.DB {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve SSH test source path")
	}
	migrationDir := filepath.Join(filepath.Dir(sourceFile), "..", "..", "sql", "schema")
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "ssh.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open SSH test database: %v", err)
	}
	db.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite"); err != nil {
		_ = db.Close()
		t.Fatalf("set sqlite dialect: %v", err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		_ = db.Close()
		t.Fatalf("apply SSH test migrations: %v", err)
	}
	return db
}

func TestClosingShellCancelsPendingFeedRefresh(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer httpServer.Close()

	db := openSSHTestDB(t)
	defer db.Close()
	feed, err := database.New(db).CreateFeed(context.Background(), database.CreateFeedParams{Name: "Blocking", Url: httpServer.URL})
	if err != nil {
		t.Fatalf("create blocking feed: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	keysPath := filepath.Join(home, "authorized_keys")
	if err := os.WriteFile(keysPath, gossh.MarshalAuthorizedKey(signer.PublicKey()), 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	config := Config{Address: listener.Addr().String(), AuthorizedKeys: keysPath, HostKey: filepath.Join(home, "host_key"), Username: "owner"}
	server, err := newServer(config, &internal.State{Db: database.New(db), SQLDB: db})
	if err != nil {
		_ = listener.Close()
		t.Fatalf("newServer() = %v", err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.serve(listener) }()
	defer func() {
		_ = server.close()
		select {
		case <-serveDone:
		case <-time.After(3 * time.Second):
			t.Error("SSH server did not stop")
		}
	}()
	client, err := gossh.Dial("tcp", listener.Addr().String(), &gossh.ClientConfig{
		User:            "owner",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	})
	if err != nil {
		t.Fatalf("SSH login: %v", err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, stdout) }()
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
		t.Fatalf("request PTY: %v", err)
	}
	if err := session.Shell(); err != nil {
		t.Fatalf("start shell: %v", err)
	}
	if _, err := io.WriteString(stdin, "\tR"); err != nil {
		t.Fatalf("request feed refresh: %v", err)
	}
	select {
	case <-requestStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("feed refresh did not start over SSH")
	}
	secondSession, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	secondOutput, err := secondSession.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(io.Discard, secondOutput) }()
	secondInput, err := secondSession.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := secondSession.RequestPty("xterm", 40, 120, gossh.TerminalModes{}); err != nil {
		t.Fatalf("request second PTY: %v", err)
	}
	if err := secondSession.Shell(); err != nil {
		t.Fatalf("start second shell: %v", err)
	}
	if err := secondSession.WindowChange(50, 130); err != nil {
		t.Fatalf("resize second shell: %v", err)
	}
	secondWait := make(chan error, 1)
	go func() { secondWait <- secondSession.Wait() }()
	if err := session.Close(); err != nil {
		t.Fatalf("close SSH shell channel: %v", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(3 * time.Second):
		t.Fatal("closing the shell channel did not cancel the feed request")
	}
	select {
	case err := <-secondWait:
		t.Fatalf("closing the first shell ended the second shell: %v", err)
	default:
	}
	if _, err := io.WriteString(secondInput, "q"); err != nil {
		t.Fatalf("quit second shell: %v", err)
	}
	select {
	case err := <-secondWait:
		if err != nil {
			t.Fatalf("second shell did not exit cleanly: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("q did not close second shell")
	}
	var lastFetched sql.NullString
	if err := db.QueryRowContext(context.Background(), `SELECT last_fetched_at FROM feeds WHERE id = ?`, feed.ID).Scan(&lastFetched); err != nil {
		t.Fatalf("read feed after cancellation: %v", err)
	}
	if lastFetched.Valid {
		t.Fatal("canceled refresh was persisted as a successful fetch")
	}
}

func TestRunStopsListeningWhenContextIsCanceled(t *testing.T) {
	home := t.TempDir()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	parsedKey, err := gossh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	keysPath := filepath.Join(home, "authorized_keys")
	if err := os.WriteFile(keysPath, gossh.MarshalAuthorizedKey(parsedKey), 0o600); err != nil {
		t.Fatal(err)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	config := Config{
		Address:        address,
		AuthorizedKeys: keysPath,
		HostKey:        filepath.Join(home, "host_key"),
		Username:       "owner",
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, config, &internal.State{}) }()

	var connection net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		connection, err = net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if connection == nil {
		cancel()
		t.Fatalf("SSH server did not start listening: %v", err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(connection)
	banner, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(banner, "SSH-2.0-") {
		t.Fatalf("read SSH identification before shutdown: banner=%q err=%v", banner, err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() after cancellation = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
	if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("incomplete SSH handshake remained open after shutdown: %v", err)
	}
	_ = connection.Close()
	if connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond); err == nil {
		_ = connection.Close()
		t.Fatal("SSH server still accepts connections after shutdown")
	}
}

func TestInteractiveShellPolicy(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		command   []string
		hasPTY    bool
		wantAllow bool
	}{
		{name: "interactive shell with pty", kind: "shell", hasPTY: true, wantAllow: true},
		{name: "shell without pty", kind: "shell"},
		{name: "exec command", kind: "exec", command: []string{"id"}, hasPTY: true},
		{name: "empty exec", kind: "exec", hasPTY: true},
		{name: "subsystem", kind: "subsystem", command: []string{"sftp"}, hasPTY: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allowInteractiveShell(test.kind, test.command, test.hasPTY); got != test.wantAllow {
				t.Fatalf("allowInteractiveShell() = %v, want %v", got, test.wantAllow)
			}
		})
	}
}
