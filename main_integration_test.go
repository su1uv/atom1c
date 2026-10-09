package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/su1uv/atom1c/internal/database"
	gossh "golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"
)

func TestExecutableFreshStartRestartPersistsDataAndHostIdentity(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	projectRoot := filepath.Dir(sourceFile)
	binary := filepath.Join(t.TempDir(), "atom1c")
	buildCtx, cancelBuild := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", binary, ".")
	build.Dir = projectRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build application executable: %v\n%s", err, output)
	}

	owner, err := user.Current()
	if err != nil {
		t.Fatalf("resolve SSH username: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create client signer: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve SSH address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release SSH address: %v", err)
	}
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	feedServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(requestStarted)
		<-request.Context().Done()
		close(requestCanceled)
	}))
	t.Cleanup(feedServer.Close)

	dataDir := t.TempDir()
	databasePath := filepath.Join(dataDir, "atom1c.db")
	keysPath := filepath.Join(dataDir, "authorized_keys")
	hostKeyPath := filepath.Join(dataDir, "ssh_host_ed25519_key")
	if err := os.WriteFile(keysPath, gossh.MarshalAuthorizedKey(signer.PublicKey()), 0o600); err != nil {
		t.Fatalf("write authorized key: %v", err)
	}

	var knownHostKey []byte
	hostKeyCallback := func(_ string, _ net.Addr, key gossh.PublicKey) error {
		encoded := key.Marshal()
		if knownHostKey != nil && !bytes.Equal(knownHostKey, encoded) {
			return fmt.Errorf("SSH host identity changed across process restart")
		}
		knownHostKey = append(knownHostKey[:0], encoded...)
		return nil
	}
	start := func(refreshInterval string) *executableServer {
		processEnvironment := executableTestEnvironment(map[string]string{
			"GOOSE_DBSTRING":          databasePath,
			"ATOM1C_SSH_ADDR":         address,
			"ATOM1C_AUTHORIZED_KEYS":  keysPath,
			"ATOM1C_SSH_HOST_KEY":     hostKeyPath,
			"ATOM1C_REFRESH_INTERVAL": refreshInterval,
		})
		output := &bytes.Buffer{}
		command := exec.Command(binary)
		command.Dir = projectRoot
		command.Env = processEnvironment
		command.Stdout = output
		command.Stderr = output
		if err := command.Start(); err != nil {
			t.Fatalf("start application executable: %v", err)
		}
		server := &executableServer{command: command, output: output, done: make(chan struct{})}
		go func() {
			server.waitErr = command.Wait()
			close(server.done)
		}()
		t.Cleanup(server.forceStop)
		return server
	}
	dial := func(server *executableServer) *gossh.Client {
		deadline := time.Now().Add(5 * time.Second)
		var lastErr error
		for time.Now().Before(deadline) {
			select {
			case <-server.done:
				t.Fatalf("application exited before SSH became ready: %v\n%s", server.waitErr, server.output.String())
			default:
			}
			client, dialErr := gossh.Dial("tcp", address, &gossh.ClientConfig{
				User:            owner.Username,
				Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
				HostKeyCallback: hostKeyCallback,
				Timeout:         250 * time.Millisecond,
			})
			if dialErr == nil {
				return client
			}
			lastErr = dialErr
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatalf("SSH did not become ready: %v\n%s", lastErr, server.output.String())
		return nil
	}

	firstProcess := start("0")
	firstClient := dial(firstProcess)
	if err := firstClient.Close(); err != nil {
		t.Fatalf("close first SSH connection: %v", err)
	}
	hostKeyBefore, err := os.ReadFile(hostKeyPath)
	if err != nil {
		t.Fatalf("read generated SSH host key: %v", err)
	}
	if mode, err := os.Stat(hostKeyPath); err != nil {
		t.Fatalf("stat generated SSH host key: %v", err)
	} else if mode.Mode().Perm()&0o077 != 0 {
		t.Fatalf("SSH host key permissions = %o, want owner-only", mode.Mode().Perm())
	}

	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open fresh application database: %v", err)
	}
	db.SetMaxOpenConns(1)
	queries := database.New(db)
	ctx := t.Context()
	var latestMigration int64
	if err := db.QueryRowContext(ctx, `SELECT max(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&latestMigration); err != nil {
		_ = db.Close()
		t.Fatalf("read embedded migration version: %v", err)
	}
	if latestMigration != 4 {
		_ = db.Close()
		t.Fatalf("latest applied migration = %d, want 4", latestMigration)
	}
	storedFeed, err := queries.CreateFeed(ctx, database.CreateFeedParams{Name: "Persisted feed", Url: feedServer.URL})
	if err != nil {
		_ = db.Close()
		t.Fatalf("persist test feed: %v", err)
	}
	storedPost, err := queries.UpsertPost(ctx, database.UpsertPostParams{
		FeedID: storedFeed.ID, IdentityKey: "test-entry", SourceID: "test-entry", Title: "Persisted post",
		Link: "https://example.test/article", Content: "Preview", ContentKind: "text", PublishedRaw: "", UpdatedRaw: "",
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("persist test post: %v", err)
	}
	if _, err := queries.UpsertArticleCache(ctx, database.UpsertArticleCacheParams{
		ID: storedPost.ID, Link: storedPost.Link, FinalUrl: storedPost.Link, Markdown: "# Cached article",
		Title: "Cached title", Author: "Author", SiteName: "Example", PublishedAt: "2026-10-09",
	}); err != nil {
		_ = db.Close()
		t.Fatalf("persist test article cache: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test database writer: %v", err)
	}
	firstProcess.stop(t)

	secondProcess := start("0")
	secondClient := dial(secondProcess)
	if err := secondClient.Close(); err != nil {
		t.Fatalf("close second SSH connection: %v", err)
	}
	hostKeyAfter, err := os.ReadFile(hostKeyPath)
	if err != nil {
		t.Fatalf("read SSH host key after restart: %v", err)
	}
	if !bytes.Equal(hostKeyBefore, hostKeyAfter) {
		t.Fatal("SSH host key file changed across process restart")
	}

	reopened, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("reopen application database: %v", err)
	}
	defer reopened.Close()
	reopened.SetMaxOpenConns(1)
	reopenedQueries := database.New(reopened)
	feeds, err := reopenedQueries.GetFeedsForRefresh(ctx)
	if err != nil {
		t.Fatalf("read feeds after restart: %v", err)
	}
	if len(feeds) != 1 || feeds[0].ID != storedFeed.ID || feeds[0].Name != "Persisted feed" {
		t.Fatalf("feeds after restart = %#v, want original feed", feeds)
	}
	posts, err := reopenedQueries.GetPostsByFeed(ctx, storedFeed.ID)
	if err != nil {
		t.Fatalf("read posts after restart: %v", err)
	}
	if len(posts) != 1 || posts[0].ID != storedPost.ID || posts[0].Title != "Persisted post" {
		t.Fatalf("posts after restart = %#v, want original post", posts)
	}
	article, err := reopenedQueries.GetArticleCache(ctx, database.GetArticleCacheParams{PostID: storedPost.ID, SourceUrl: storedPost.Link})
	if err != nil {
		t.Fatalf("read cached article after restart: %v", err)
	}
	if article.Markdown != "# Cached article" || article.Title != "Cached title" {
		t.Fatalf("cached article after restart = %#v", article)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close verification database: %v", err)
	}
	secondProcess.stop(t)

	thirdProcess := start("1h")
	select {
	case <-requestStarted:
	case <-thirdProcess.done:
		t.Fatalf("server exited before scheduled feed refresh: %v\n%s", thirdProcess.waitErr, thirdProcess.output.String())
	case <-time.After(5 * time.Second):
		t.Fatal("scheduled feed refresh did not start")
	}
	thirdProcess.stop(t)
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("server returned before canceling the active feed request")
	}

	afterShutdown, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open database after process shutdown: %v", err)
	}
	defer afterShutdown.Close()
	var persistedTitle string
	if err := afterShutdown.QueryRowContext(ctx, `SELECT title FROM posts WHERE id = ?`, storedPost.ID).Scan(&persistedTitle); err != nil {
		t.Fatalf("read database after active refresh cancellation: %v", err)
	}
	if persistedTitle != "Persisted post" {
		t.Fatalf("post after active refresh cancellation = %q, want persisted value", persistedTitle)
	}
}

type executableServer struct {
	command *exec.Cmd
	output  *bytes.Buffer
	done    chan struct{}
	waitErr error
}

func (server *executableServer) stop(t *testing.T) {
	t.Helper()
	if server.command.Process == nil {
		return
	}
	_ = server.command.Process.Signal(os.Interrupt)
	select {
	case <-server.done:
		if server.waitErr != nil {
			t.Fatalf("stop server: %v\n%s", server.waitErr, server.output.String())
		}
	case <-time.After(5 * time.Second):
		_ = server.command.Process.Kill()
		<-server.done
		t.Fatalf("server did not stop after SIGINT\n%s", server.output.String())
	}
}

func (server *executableServer) forceStop() {
	if server.command.Process == nil {
		return
	}
	select {
	case <-server.done:
		return
	default:
	}
	_ = server.command.Process.Kill()
	select {
	case <-server.done:
	case <-time.After(2 * time.Second):
	}
}

func executableTestEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := overrides[name]; !replaced {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}
