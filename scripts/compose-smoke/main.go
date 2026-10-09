// Command compose-smoke runs the opt-in Docker Compose end-to-end workflow.
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/su1uv/atom1c/internal/database"
	gossh "golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"
)

const (
	sshUser          = "atom1c"
	serviceName      = "atom1c"
	sshContainerPort = 23234
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "compose smoke:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("resolve smoke-test source path")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(sourceFile)))
	workDir, err := os.MkdirTemp("", "atom1c-compose-smoke-")
	if err != nil {
		return fmt.Errorf("create smoke-test work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate SSH client key: %w", err)
	}
	signer, err := gossh.NewSignerFromKey(privateKey)
	if err != nil {
		return fmt.Errorf("create SSH signer: %w", err)
	}
	keysPath := filepath.Join(workDir, "authorized_keys")
	if err := os.WriteFile(keysPath, gossh.MarshalAuthorizedKey(signer.PublicKey()), 0o644); err != nil {
		return fmt.Errorf("write public authorized key: %w", err)
	}
	project := fmt.Sprintf("atom1c-smoke-%d-%d", os.Getpid(), time.Now().UnixNano())
	compose := composeRunner{
		ctx:          ctx,
		root:         projectRoot,
		project:      project,
		authorized:   keysPath,
		appImage:     project + ":local",
		fixtureImage: project + "-fixture:local",
	}
	defer func() {
		cleanup := composeRunner{
			root: compose.root, project: compose.project, authorized: compose.authorized,
			appImage: compose.appImage, fixtureImage: compose.fixtureImage,
		}
		if resultErr != nil {
			if logs, logsErr := cleanup.command("1h", "logs", "--no-color", "--tail", "100", serviceName); logsErr == nil {
				fmt.Fprintln(os.Stderr, "container logs:\n"+logs)
			}
		}
		output, err := cleanup.command("1h", "down", "--volumes", "--remove-orphans")
		if err != nil {
			cleanupErr := fmt.Errorf("remove smoke-test Compose project: %w\n%s", err, output)
			resultErr = errors.Join(resultErr, cleanupErr)
		}
		removeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = exec.CommandContext(removeCtx, "docker", "rm", "--force", compose.project+"-restore").Run()
		_ = exec.CommandContext(removeCtx, "docker", "image", "rm", "--force", compose.appImage, compose.fixtureImage).Run()
	}()

	feedBaseURL := "http://feed-fixture:8080"

	if err := compose.checkMissingAuthorizedKeyEnvironment(); err != nil {
		return err
	}
	if output, err := compose.command("1h", "build"); err != nil {
		return fmt.Errorf("build application image: %w\n%s", err, output)
	}
	if err := compose.checkMissingAuthorizedKeyFile(); err != nil {
		return err
	}
	if output, err := compose.command("1h", "up", "--detach", "--no-build"); err != nil {
		return fmt.Errorf("start application container: %w\n%s", err, output)
	}

	containerID, err := compose.containerID("1h")
	if err != nil {
		return err
	}
	sshPort, err := verifyContainerConfiguration(ctx, containerID)
	if err != nil {
		return err
	}
	fixtureID, err := compose.serviceContainerID("1h", "feed-fixture")
	if err != nil {
		return err
	}
	fixturePort, err := verifyPublishedPort(ctx, fixtureID, 8080)
	if err != nil {
		return err
	}

	var hostKeyMu sync.Mutex
	var hostKey []byte
	hostKeyCallback := func(_ string, _ net.Addr, key gossh.PublicKey) error {
		hostKeyMu.Lock()
		defer hostKeyMu.Unlock()
		encoded := key.Marshal()
		if hostKey != nil && !bytes.Equal(hostKey, encoded) {
			return errors.New("SSH host identity changed after container recreation")
		}
		hostKey = append(hostKey[:0], encoded...)
		return nil
	}
	client, err := compose.waitForSSH(sshPort, signer, hostKeyCallback)
	if err != nil {
		return fmt.Errorf("connect after initial container startup: %w", err)
	}

	atomSession, err := addAndRefreshFeed(client, compose, workDir, "Smoke Atom", feedBaseURL+"/atom.xml", "ATOM_V1")
	if err != nil {
		_ = client.Close()
		return err
	}
	rssSession, err := addAndRefreshFeed(client, compose, workDir, "Smoke RSS", feedBaseURL+"/rss.xml", "RSS_V1")
	if err != nil {
		_ = atomSession.close()
		_ = client.Close()
		return err
	}

	secondSession, err := openTerminal(ctx, client)
	if err != nil {
		_ = atomSession.close()
		_ = rssSession.close()
		_ = client.Close()
		return err
	}
	if err := secondSession.waitForText("Smoke Atom", 10*time.Second); err != nil {
		return fmt.Errorf("load feeds in second SSH session: %w", err)
	}
	if err := secondSession.send("\t"); err != nil {
		return err
	}
	if err := secondSession.waitForText("ATOM_V1", 10*time.Second); err != nil {
		return fmt.Errorf("open the same feed in a second SSH session: %w", err)
	}

	if err := setFixtureTitle(ctx, fixturePort, "atom", "UPDATED_V2"); err != nil {
		return err
	}
	if err := atomSession.send("R"); err != nil {
		return err
	}
	containerID, err = compose.containerID("1h")
	if err != nil {
		return err
	}
	if _, err := compose.waitForDatabase(containerID, workDir, func(snapshot databaseSnapshot) bool {
		return snapshot.hasPost("Smoke Atom", "UPDATED_V2") && snapshot.postCount("Smoke Atom") == 1
	}); err != nil {
		return fmt.Errorf("verify manual refresh, shared data, and deduplication: %w", err)
	}
	if err := atomSession.waitForText("UPDATED_V2", 10*time.Second); err != nil {
		return fmt.Errorf("observe refreshed post in initiating session: %w", err)
	}
	if err := secondSession.waitForText("UPDATED_V2", 10*time.Second); err != nil {
		return fmt.Errorf("observe shared refresh in second session: %w", err)
	}

	initialContainerID := containerID
	for _, terminal := range []*terminal{atomSession, rssSession, secondSession} {
		if err := terminal.close(); err != nil {
			return fmt.Errorf("close SSH reader session: %w", err)
		}
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close SSH connection before offline refresh: %w", err)
	}

	if err := setFixtureTitle(ctx, fixturePort, "atom", "SCHEDULED_V3"); err != nil {
		return err
	}
	if err := setFixtureTitle(ctx, fixturePort, "rss", "SCHEDULED_RSS_V2"); err != nil {
		return err
	}
	if output, err := compose.command("1s", "up", "--detach", "--no-build", "--force-recreate", serviceName); err != nil {
		return fmt.Errorf("recreate container with scheduled refresh enabled: %w\n%s", err, output)
	}
	containerID, err = compose.containerID("1s")
	if err != nil {
		return err
	}
	if containerID == initialContainerID {
		return errors.New("Compose did not recreate the application container")
	}
	sshPort, err = verifyContainerConfiguration(ctx, containerID)
	if err != nil {
		return err
	}
	if _, err := compose.waitForDatabase(containerID, workDir, func(snapshot databaseSnapshot) bool {
		return snapshot.hasPost("Smoke Atom", "SCHEDULED_V3") &&
			snapshot.hasPost("Smoke RSS", "SCHEDULED_RSS_V2") &&
			snapshot.postCount("Smoke Atom") == 1 && snapshot.postCount("Smoke RSS") == 1 &&
			snapshot.hasSuccessfulFetch("Smoke Atom") && snapshot.hasSuccessfulFetch("Smoke RSS")
	}); err != nil {
		return fmt.Errorf("verify automatic refresh persisted with no connected SSH clients: %w", err)
	}
	if err := verifyPersistentHostKey(ctx, containerID, workDir); err != nil {
		return err
	}

	client, err = compose.waitForSSH(sshPort, signer, hostKeyCallback)
	if err != nil {
		return fmt.Errorf("reconnect after container recreation: %w", err)
	}
	postRestartSession, err := openTerminal(ctx, client)
	if err != nil {
		_ = client.Close()
		return err
	}
	if err := postRestartSession.waitForText("Smoke Atom", 10*time.Second); err != nil {
		_ = postRestartSession.close()
		_ = client.Close()
		return fmt.Errorf("load feeds after container recreation: %w", err)
	}
	if err := postRestartSession.send("\t"); err != nil {
		_ = postRestartSession.close()
		_ = client.Close()
		return err
	}
	if err := postRestartSession.waitForText("SCHEDULED_V3", 10*time.Second); err != nil {
		return fmt.Errorf("read persisted post after container recreation: %w", err)
	}
	if err := postRestartSession.close(); err != nil {
		return fmt.Errorf("close post-restart reader session: %w", err)
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close post-restart SSH connection: %w", err)
	}
	if err := setFixtureBlocked(ctx, fixturePort, true); err != nil {
		return err
	}
	if err := waitForFixtureRequest(ctx, fixturePort, true); err != nil {
		return fmt.Errorf("wait for scheduled refresh to enter the blocking fixture: %w", err)
	}
	if output, err := compose.command("1s", "stop", "--timeout", "20", serviceName); err != nil {
		return fmt.Errorf("gracefully stop container: %w\n%s", err, output)
	}
	if err := waitForFixtureRequest(ctx, fixturePort, false); err != nil {
		return fmt.Errorf("verify container shutdown canceled the active feed request: %w", err)
	}

	containerID, err = compose.containerID("1s")
	if err != nil {
		return err
	}
	databaseBackup := filepath.Join(workDir, "atom1c.db.backup")
	if output, err := compose.command("1s", "cp", serviceName+":/data/atom1c.db", databaseBackup); err != nil {
		return fmt.Errorf("back up database with Compose: %w\n%s", err, output)
	}
	hostKeyBackup := filepath.Join(workDir, "ssh_host_ed25519_key.backup")
	if output, err := compose.command("1s", "cp", serviceName+":/data/ssh_host_ed25519_key", hostKeyBackup); err != nil {
		return fmt.Errorf("back up SSH host key with Compose: %w\n%s", err, output)
	}
	keyInfo, err := os.Stat(hostKeyBackup)
	if err != nil {
		return fmt.Errorf("stat SSH host-key backup: %w", err)
	}
	if keyInfo.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("SSH host-key backup permissions = %o, want owner-only", keyInfo.Mode().Perm())
	}
	snapshot, err := readDatabaseSnapshot(ctx, databaseBackup)
	if err != nil {
		return fmt.Errorf("read final stopped-container database backup: %w", err)
	}
	if !snapshot.hasPost("Smoke Atom", "SCHEDULED_V3") || !snapshot.hasPost("Smoke RSS", "SCHEDULED_RSS_V2") {
		return fmt.Errorf("final database posts do not contain the expected Atom/RSS updates: %#v", snapshot.posts)
	}
	if snapshot.postCount("Smoke Atom") != 1 || snapshot.postCount("Smoke RSS") != 1 {
		return fmt.Errorf("duplicate posts after repeated and scheduled refreshes: %#v", snapshot.posts)
	}
	if snapshot.migration != 4 {
		return fmt.Errorf("final migration version = %d, want 4", snapshot.migration)
	}
	if err := setFixtureBlocked(ctx, fixturePort, false); err != nil {
		return err
	}
	if err := restoreDatabaseBackup(ctx, containerID, workDir, project); err != nil {
		return err
	}
	if output, err := compose.command("1s", "start", serviceName); err != nil {
		return fmt.Errorf("start service after backup restore: %w\n%s", err, output)
	}
	containerID, err = compose.containerID("1s")
	if err != nil {
		return err
	}
	sshPort, err = verifyContainerConfiguration(ctx, containerID)
	if err != nil {
		return err
	}
	client, err = compose.waitForSSH(sshPort, signer, hostKeyCallback)
	if err != nil {
		return fmt.Errorf("reconnect after backup restore: %w", err)
	}
	restoredSession, err := openTerminal(ctx, client)
	if err != nil {
		_ = client.Close()
		return err
	}
	if err := restoredSession.waitForText("Smoke Atom", 10*time.Second); err != nil {
		_ = restoredSession.close()
		_ = client.Close()
		return fmt.Errorf("load feeds from restored database: %w", err)
	}
	if err := restoredSession.send("\t"); err != nil {
		_ = restoredSession.close()
		_ = client.Close()
		return err
	}
	if err := restoredSession.waitForText("SCHEDULED_V3", 10*time.Second); err != nil {
		_ = restoredSession.close()
		_ = client.Close()
		return fmt.Errorf("read restored post: %w", err)
	}
	if err := restoredSession.close(); err != nil {
		return fmt.Errorf("close reader after backup restore: %w", err)
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close SSH connection after backup restore: %w", err)
	}
	if output, err := compose.command("1s", "stop", "--timeout", "20", serviceName); err != nil {
		return fmt.Errorf("stop service after backup restore: %w\n%s", err, output)
	}
	fmt.Println("Docker Compose smoke passed: non-root SSH, Atom/RSS refresh and deduplication, concurrent session reload, no-client scheduling, active-request shutdown, volume/host-key persistence, backup/restore, and graceful stop.")
	return nil
}

type composeRunner struct {
	ctx          context.Context
	root         string
	project      string
	authorized   string
	appImage     string
	fixtureImage string
}

func (runner composeRunner) args() []string {
	return []string{
		"compose", "--env-file", "/dev/null", "--project-name", runner.project,
		"--file", filepath.Join(runner.root, "compose.yaml"),
		"--file", filepath.Join(runner.root, "compose.smoke.yaml"),
	}
}

func (runner composeRunner) command(interval string, args ...string) (string, error) {
	return runner.commandWithContext(runner.ctx, interval, args...)
}

func (runner composeRunner) commandWithContext(parent context.Context, interval string, args ...string) (string, error) {
	if parent == nil {
		parent = context.Background()
	}
	commandArgs := append(runner.args(), args...)
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", commandArgs...)
	command.Dir = runner.root
	command.Env = replaceEnvironment(map[string]string{
		"ATOM1C_AUTHORIZED_KEYS":     runner.authorized,
		"ATOM1C_SSH_PORT":            "0",
		"ATOM1C_SSH_BIND_ADDRESS":    "127.0.0.1",
		"ATOM1C_REFRESH_INTERVAL":    interval,
		"ATOM1C_FIXTURE_PORT":        "0",
		"ATOM1C_SMOKE_IMAGE":         runner.appImage,
		"ATOM1C_SMOKE_FIXTURE_IMAGE": runner.fixtureImage,
	})
	output, err := command.CombinedOutput()
	return string(output), err
}

func (runner composeRunner) checkMissingAuthorizedKeyEnvironment() error {
	commandArgs := append(runner.args(), "config", "--quiet")
	parent := runner.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", commandArgs...)
	command.Dir = runner.root
	command.Env = replaceEnvironment(map[string]string{
		"ATOM1C_AUTHORIZED_KEYS":     "",
		"ATOM1C_SSH_PORT":            "0",
		"ATOM1C_SSH_BIND_ADDRESS":    "127.0.0.1",
		"ATOM1C_REFRESH_INTERVAL":    "1h",
		"ATOM1C_FIXTURE_PORT":        "0",
		"ATOM1C_SMOKE_IMAGE":         runner.appImage,
		"ATOM1C_SMOKE_FIXTURE_IMAGE": runner.fixtureImage,
	})
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Set ATOM1C_AUTHORIZED_KEYS") {
		return fmt.Errorf("missing authorized-key configuration did not fail clearly: %v\n%s", err, output)
	}
	return nil
}

func (runner composeRunner) checkMissingAuthorizedKeyFile() error {
	missingPath := filepath.Join(filepath.Dir(runner.authorized), "missing-authorized-keys")
	command := composeRunner{
		ctx: runner.ctx, root: runner.root, project: runner.project, authorized: missingPath,
		appImage: runner.appImage, fixtureImage: runner.fixtureImage,
	}
	output, err := command.command("1h", "create", "--no-build", serviceName)
	if err == nil || !(strings.Contains(strings.ToLower(output), "no such file") || strings.Contains(strings.ToLower(output), "not exist")) {
		return fmt.Errorf("missing authorized-key file did not fail clearly: %v\n%s", err, output)
	}
	return nil
}

func (runner composeRunner) containerID(interval string) (string, error) {
	return runner.serviceContainerID(interval, serviceName)
}

func (runner composeRunner) serviceContainerID(interval, service string) (string, error) {
	output, err := runner.command(interval, "ps", "--all", "--quiet", service)
	if err != nil {
		return "", fmt.Errorf("inspect Compose service %q: %w\n%s", service, err, output)
	}
	id := strings.TrimSpace(output)
	if id == "" {
		return "", fmt.Errorf("Compose did not report a container ID for service %q", service)
	}
	return id, nil
}

func (runner composeRunner) waitForSSH(port int, signer gossh.Signer, hostKeyCallback gossh.HostKeyCallback) (*gossh.Client, error) {
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := runner.ctx.Err(); err != nil {
			return nil, err
		}
		client, err := gossh.Dial("tcp", address, &gossh.ClientConfig{
			User:            sshUser,
			Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
			HostKeyCallback: hostKeyCallback,
			Timeout:         2 * time.Second,
		})
		if err == nil {
			return client, nil
		}
		lastErr = err
		if strings.Contains(err.Error(), "SSH host identity changed") {
			return nil, err
		}
		if !waitContext(runner.ctx, 100*time.Millisecond) {
			return nil, runner.ctx.Err()
		}
	}
	logs, _ := runner.command("1h", "logs", "--no-color", "--tail", "100", serviceName)
	return nil, fmt.Errorf("SSH did not become ready: %v\n%s", lastErr, logs)
}

func (runner composeRunner) waitForDatabase(containerID, workDir string, accept func(databaseSnapshot) bool) (databaseSnapshot, error) {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := runner.ctx.Err(); err != nil {
			return databaseSnapshot{}, err
		}
		snapshot, err := runner.copyDatabase(containerID, workDir)
		if err == nil && accept(snapshot) {
			return snapshot, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("current database state: %#v", snapshot.posts)
		}
		if !waitContext(runner.ctx, 150*time.Millisecond) {
			return databaseSnapshot{}, runner.ctx.Err()
		}
	}
	return databaseSnapshot{}, fmt.Errorf("database did not reach expected state: %w", lastErr)
}

func (runner composeRunner) copyDatabase(containerID, workDir string) (databaseSnapshot, error) {
	path := filepath.Join(workDir, "snapshot.db")
	_ = os.Remove(path)
	parent := runner.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	copy := exec.CommandContext(ctx, "docker", "cp", containerID+":/data/atom1c.db", path)
	output, err := copy.CombinedOutput()
	if err != nil {
		return databaseSnapshot{}, fmt.Errorf("docker cp database: %w: %s", err, output)
	}
	return readDatabaseSnapshot(runner.ctx, path)
}

func readDatabaseSnapshot(parent context.Context, path string) (databaseSnapshot, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return databaseSnapshot{}, fmt.Errorf("open copied SQLite database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	queries := database.New(db)
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	feeds, err := queries.GetFeedsForRefresh(ctx)
	if err != nil {
		return databaseSnapshot{}, fmt.Errorf("read copied feeds: %w", err)
	}
	snapshot := databaseSnapshot{
		posts:   make(map[string][]string, len(feeds)),
		fetched: make(map[string]bool, len(feeds)),
		feedIDs: make(map[string]int64, len(feeds)),
	}
	for _, feed := range feeds {
		snapshot.feedIDs[feed.Name] = feed.ID
		snapshot.fetched[feed.Name] = feed.LastFetchedAt.Valid
		posts, err := queries.GetPostsByFeed(ctx, feed.ID)
		if err != nil {
			return databaseSnapshot{}, fmt.Errorf("read copied posts for %q: %w", feed.Name, err)
		}
		for _, post := range posts {
			snapshot.posts[feed.Name] = append(snapshot.posts[feed.Name], post.Title)
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT max(version_id) FROM goose_db_version WHERE is_applied = 1`).Scan(&snapshot.migration); err != nil {
		return databaseSnapshot{}, fmt.Errorf("read copied migration version: %w", err)
	}
	return snapshot, nil
}

type databaseSnapshot struct {
	posts     map[string][]string
	fetched   map[string]bool
	feedIDs   map[string]int64
	migration int64
}

func (snapshot databaseSnapshot) hasPost(feed, title string) bool {
	for _, got := range snapshot.posts[feed] {
		if got == title {
			return true
		}
	}
	return false
}

func (snapshot databaseSnapshot) postCount(feed string) int {
	return len(snapshot.posts[feed])
}

func (snapshot databaseSnapshot) hasSuccessfulFetch(feed string) bool {
	return snapshot.fetched[feed]
}

func addAndRefreshFeed(client *gossh.Client, compose composeRunner, workDir, name, url, title string) (*terminal, error) {
	terminal, err := openTerminal(compose.ctx, client)
	if err != nil {
		return nil, err
	}
	if err := terminal.send("a" + name + "\t" + url + "\t\r"); err != nil {
		_ = terminal.close()
		return nil, err
	}
	containerID, err := compose.containerID("1h")
	if err != nil {
		_ = terminal.close()
		return nil, err
	}
	if _, err := compose.waitForDatabase(containerID, workDir, func(snapshot databaseSnapshot) bool {
		return snapshot.migration == 4 && snapshot.feedIDs[name] != 0
	}); err != nil {
		_ = terminal.close()
		return nil, fmt.Errorf("add feed %q through SSH: %w", name, err)
	}
	if err := terminal.send("\tR"); err != nil {
		_ = terminal.close()
		return nil, err
	}
	if _, err := compose.waitForDatabase(containerID, workDir, func(snapshot databaseSnapshot) bool {
		return snapshot.hasPost(name, title) && snapshot.postCount(name) == 1 && snapshot.hasSuccessfulFetch(name)
	}); err != nil {
		_ = terminal.close()
		return nil, fmt.Errorf("refresh and persist feed %q through SSH: %w\nterminal output:\n%s", name, err, terminal.output.String())
	}
	if err := terminal.waitForText(title, 10*time.Second); err != nil {
		_ = terminal.close()
		return nil, fmt.Errorf("display post for feed %q: %w", name, err)
	}
	return terminal, nil
}

func openTerminal(ctx context.Context, client *gossh.Client) (*terminal, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("open SSH session: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("open SSH output: %w", err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("open SSH input: %w", err)
	}
	if err := session.RequestPty("xterm-256color", 30, 100, gossh.TerminalModes{}); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("request SSH PTY: %w", err)
	}
	if err := session.Shell(); err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("start SSH interactive shell: %w", err)
	}
	terminal := &terminal{ctx: ctx, session: session, stdin: stdin, output: &lockedBuffer{}, done: make(chan error, 1)}
	go func() { _, _ = io.Copy(terminal.output, stdout) }()
	go func() { terminal.done <- session.Wait() }()
	return terminal, nil
}

type terminal struct {
	ctx     context.Context
	session *gossh.Session
	stdin   io.WriteCloser
	output  *lockedBuffer
	done    chan error
}

func (terminal *terminal) send(keys string) error {
	if _, err := io.WriteString(terminal.stdin, keys); err != nil {
		return fmt.Errorf("send keys to SSH reader: %w", err)
	}
	return nil
}

func (terminal *terminal) waitForText(text string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := terminal.ctx.Err(); err != nil {
			return err
		}
		if strings.Contains(ansi.Strip(terminal.output.String()), text) {
			return nil
		}
		select {
		case <-terminal.ctx.Done():
			return terminal.ctx.Err()
		case err := <-terminal.done:
			return fmt.Errorf("SSH reader ended before displaying %q: %v", text, err)
		default:
		}
		if !waitContext(terminal.ctx, 50*time.Millisecond) {
			return terminal.ctx.Err()
		}
	}
	return fmt.Errorf("timed out waiting for %q in terminal output: %q", text, terminal.output.String())
}

func (terminal *terminal) close() error {
	if err := terminal.ctx.Err(); err != nil {
		_ = terminal.session.Close()
		return nil
	}
	_ = terminal.send("q")
	select {
	case err := <-terminal.done:
		if err != nil {
			return fmt.Errorf("wait for SSH reader to quit: %w", err)
		}
	case <-time.After(5 * time.Second):
		_ = terminal.session.Close()
		return errors.New("SSH reader did not quit after q")
	}
	_ = terminal.session.Close()
	return nil
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *lockedBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(value)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func setFixtureTitle(parent context.Context, port int, format, title string) error {
	path := map[string]string{"atom": "/set/atom", "rss": "/set/rss"}[format]
	if path == "" {
		return fmt.Errorf("unsupported fixture format %q", format)
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://127.0.0.1:"+strconv.Itoa(port)+path+"?title="+url.QueryEscape(title), nil)
	if err != nil {
		return fmt.Errorf("create feed-fixture update request: %w", err)
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("update %s feed fixture: %w", format, err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("update %s feed fixture returned %s", format, response.Status)
	}
	return nil
}

func setFixtureBlocked(parent context.Context, port int, blocked bool) error {
	value := strconv.FormatBool(blocked)
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://127.0.0.1:"+strconv.Itoa(port)+"/set/block-atom?enabled="+value, nil)
	if err != nil {
		return fmt.Errorf("create blocked-feed fixture request: %w", err)
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("configure blocked-feed fixture: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("configure blocked-feed fixture returned %s", response.Status)
	}
	return nil
}

func waitForFixtureRequest(ctx context.Context, port int, waitingForActive bool) error {
	deadline := time.Now().Add(10 * time.Second)
	var lastState struct {
		AtomActive   int `json:"atom_active"`
		AtomCanceled int `json:"atom_canceled"`
	}
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		requestCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/state", nil)
		if err == nil {
			response, requestErr := (&http.Client{Timeout: 2 * time.Second}).Do(request)
			if requestErr == nil {
				decodeErr := json.NewDecoder(response.Body).Decode(&lastState)
				_ = response.Body.Close()
				if decodeErr == nil {
					if waitingForActive && lastState.AtomActive > 0 || !waitingForActive && lastState.AtomCanceled > 0 {
						cancel()
						return nil
					}
				}
			}
		}
		cancel()
		if !waitContext(ctx, 100*time.Millisecond) {
			return ctx.Err()
		}
	}
	return fmt.Errorf("timed out waiting for fixture request state (active=%d canceled=%d)", lastState.AtomActive, lastState.AtomCanceled)
}

func replaceEnvironment(overrides map[string]string) []string {
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := overrides[name]; !replaced {
				environment = append(environment, entry)
			}
		}
	}
	for name, value := range overrides {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func verifyContainerConfiguration(parent context.Context, containerID string) (int, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("inspect application container: %w", err)
	}
	var records []struct {
		Config struct {
			User string
		}
		HostConfig struct {
			ReadonlyRootfs bool
		}
		Mounts []struct {
			Type        string
			Name        string
			Destination string
			RW          bool
		}
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string
			} `json:"Ports"`
		}
	}
	if err := json.Unmarshal(output, &records); err != nil {
		return 0, fmt.Errorf("decode container inspection: %w", err)
	}
	if len(records) != 1 {
		return 0, fmt.Errorf("container inspection returned %d records", len(records))
	}
	container := records[0]
	if container.Config.User != "10001:10001" || !container.HostConfig.ReadonlyRootfs {
		return 0, fmt.Errorf("container security settings: user=%q read-only-rootfs=%t", container.Config.User, container.HostConfig.ReadonlyRootfs)
	}
	dataVolume, keyMount := false, false
	for _, mount := range container.Mounts {
		if mount.Destination == "/data" {
			dataVolume = mount.Type == "volume" && mount.Name != "" && mount.RW
		}
		if mount.Destination == "/run/secrets/authorized_keys" {
			keyMount = mount.Type == "bind" && !mount.RW
		}
	}
	if !dataVolume || !keyMount {
		return 0, fmt.Errorf("container mounts must include writable named /data and read-only authorized_keys bind mount (data=%t keys=%t)", dataVolume, keyMount)
	}
	portBindings := container.NetworkSettings.Ports[fmt.Sprintf("%d/tcp", sshContainerPort)]
	if len(portBindings) != 1 || portBindings[0].HostIP != "127.0.0.1" {
		return 0, fmt.Errorf("SSH host binding = %#v, want one loopback binding", portBindings)
	}
	port, err := strconv.Atoi(portBindings[0].HostPort)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid published SSH host port %q", portBindings[0].HostPort)
	}
	return port, nil
}

func verifyPublishedPort(parent context.Context, containerID string, containerPort int) (int, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", "inspect", containerID)
	output, err := command.Output()
	if err != nil {
		return 0, fmt.Errorf("inspect published fixture port: %w", err)
	}
	var records []struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string
			} `json:"Ports"`
		}
	}
	if err := json.Unmarshal(output, &records); err != nil {
		return 0, fmt.Errorf("decode fixture container inspection: %w", err)
	}
	if len(records) != 1 {
		return 0, fmt.Errorf("fixture container inspection returned %d records", len(records))
	}
	bindings := records[0].NetworkSettings.Ports[fmt.Sprintf("%d/tcp", containerPort)]
	if len(bindings) != 1 || bindings[0].HostIP != "127.0.0.1" {
		return 0, fmt.Errorf("fixture host binding = %#v, want one loopback binding", bindings)
	}
	port, err := strconv.Atoi(bindings[0].HostPort)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid published fixture host port %q", bindings[0].HostPort)
	}
	return port, nil
}

func verifyPersistentHostKey(ctx context.Context, containerID, workDir string) error {
	path := filepath.Join(workDir, "host-key")
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", "cp", containerID+":/data/ssh_host_ed25519_key", path)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy persistent SSH host key: %w: %s", err, output)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat persistent SSH host key: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("persistent SSH host key permissions = %o, want owner-only", info.Mode().Perm())
	}
	return nil
}

func restoreDatabaseBackup(parent context.Context, containerID, workDir, project string) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	inspect := exec.CommandContext(ctx, "docker", "inspect", "--format",
		`{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}`, containerID)
	volumeOutput, err := inspect.Output()
	if err != nil {
		return fmt.Errorf("find Compose data volume for restore: %w", err)
	}
	volume := strings.TrimSpace(string(volumeOutput))
	if volume == "" {
		return errors.New("container has no named /data volume to restore")
	}
	copy := `cp /backup/atom1c.db.backup /data/atom1c.db
cp /backup/ssh_host_ed25519_key.backup /data/ssh_host_ed25519_key
rm -f /data/atom1c.db-wal /data/atom1c.db-shm
chown 10001:10001 /data/atom1c.db /data/ssh_host_ed25519_key
chmod 600 /data/atom1c.db /data/ssh_host_ed25519_key`
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", project+"-restore", "--network", "none",
		"--volume", volume+":/data", "--volume", workDir+":/backup:ro",
		"busybox:1.37.0", "sh", "-ec", copy)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore database and host identity into named volume: %w\n%s", err, output)
	}
	return nil
}

func waitContext(ctx context.Context, interval time.Duration) bool {
	if ctx == nil {
		time.Sleep(interval)
		return true
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
