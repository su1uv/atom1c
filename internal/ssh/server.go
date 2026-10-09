package ssh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	charmssh "charm.land/ssh"
	"charm.land/wish/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/su1uv/atom1c/internal"
	"github.com/su1uv/atom1c/internal/ui"
)

const serverShutdownTimeout = 10 * time.Second
const serverHandshakeTimeout = 10 * time.Second

type sessionPTY struct {
	pty           charmssh.Pty
	windowChanges <-chan charmssh.Window
	environment   []string
	emulated      bool
}

// server wraps a Wish SSH server configured for Atom1c's interactive reader.
type server struct {
	server      *charmssh.Server
	connections *connectionTracker
}

// newServer loads authentication and host identity once, then constructs the
// SSH server. Every accepted session receives an independent TUI model backed
// by the shared application state.
func newServer(config Config, state *internal.State) (*server, error) {
	if config.Username == "" {
		return nil, errors.New("SSH owner username is empty")
	}
	if err := validateListenAddress(config.Address); err != nil {
		return nil, fmt.Errorf("invalid SSH listen address: %w", err)
	}
	keys, err := loadAuthorizedKeyFile(config.AuthorizedKeys)
	if err != nil {
		return nil, err
	}
	if _, err := loadHostSigner(config.HostKey); err != nil {
		return nil, err
	}

	connections := newConnectionTracker()
	var sessionPTYs sync.Map
	wishServer, err := wish.NewServer(
		wish.WithAddress(config.Address),
		wish.WithHostKeyPath(config.HostKey),
		wish.WithPublicKeyAuth(func(ctx charmssh.Context, key charmssh.PublicKey) bool {
			return keys.Allows(config.Username, ctx.User(), key)
		}),
		wish.WithMiddleware(sessionMiddleware(state, &sessionPTYs)),
	)
	if err != nil {
		return nil, fmt.Errorf("configure SSH server: %w", err)
	}
	wishServer.ChannelHandlers = map[string]charmssh.ChannelHandler{
		"session": charmssh.DefaultSessionHandler,
	}
	wishServer.RequestHandlers = map[string]charmssh.RequestHandler{}
	wishServer.SubsystemHandlers = map[string]charmssh.SubsystemHandler{}
	wishServer.LocalPortForwardingCallback = func(charmssh.Context, string, uint32) bool { return false }
	wishServer.ReversePortForwardingCallback = func(charmssh.Context, string, uint32) bool { return false }
	wishServer.HandshakeTimeout = serverHandshakeTimeout
	wishServer.SessionRequestCallback = func(session charmssh.Session, requestType string) bool {
		pty, windowChanges, hasPTY := session.Pty()
		allowed := allowInteractiveShell(requestType, session.Command(), hasPTY)
		if allowed {
			id, ok := sessionIdentity(session)
			if !ok {
				return false
			}
			sessionPTYs.Store(id, sessionPTY{
				pty:           pty,
				windowChanges: windowChanges,
				environment:   session.Environ(),
				emulated:      session.EmulatedPty(),
			})
		}
		return allowed
	}
	return &server{server: wishServer, connections: connections}, nil
}

func sessionMiddleware(state *internal.State, ptys *sync.Map) wish.Middleware {
	return func(next charmssh.Handler) charmssh.Handler {
		return func(session charmssh.Session) {
			id, validID := sessionIdentity(session)
			value, ok := ptys.LoadAndDelete(id)
			ptyConfig, validConfig := value.(sessionPTY)
			if !validID || !ok || !validConfig {
				wish.Fatalln(session, "interactive PTY window stream unavailable")
				return
			}

			sessionCtx, cancel := context.WithCancel(session.Context())
			program := tea.NewProgram(
				ui.NewModel(state, sessionCtx),
				sessionProgramOptions(session, ptyConfig)...,
			)
			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						slog.Error("panic in SSH session resize handler", "panic", recovered, "stack", string(debug.Stack()))
						program.Quit()
					}
				}()
				for {
					select {
					case <-sessionCtx.Done():
						program.Quit()
						return
					case window, open := <-ptyConfig.windowChanges:
						if !open {
							cancel()
							program.Quit()
							return
						}
						program.Send(tea.WindowSizeMsg{Width: window.Width, Height: window.Height})
					}
				}
			}()
			if _, err := program.Run(); err != nil {
				slog.Error("SSH reader session exited with error", "error", err)
			}
			cancel()
			program.Kill()
			next(session)
		}
	}
}

func sessionIdentity(session charmssh.Session) (uintptr, bool) {
	value := reflect.ValueOf(session)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return 0, false
	}
	return value.Pointer(), true
}

func sessionProgramOptions(session charmssh.Session, ptyConfig sessionPTY) []tea.ProgramOption {
	environment := append(ptyConfig.environment, "TERM="+ptyConfig.pty.Term)
	options := []tea.ProgramOption{
		tea.WithInput(session),
		tea.WithOutput(session),
		tea.WithEnvironment(environment),
		tea.WithWindowSize(int(ptyConfig.pty.Window.Width), int(ptyConfig.pty.Window.Height)),
		tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if _, ok := msg.(tea.SuspendMsg); ok {
				return tea.ResumeMsg{}
			}
			return msg
		}),
	}
	if ptyConfig.emulated {
		options = append(options, tea.WithColorProfile(colorprofile.Env(environment)))
	}
	return options
}

func allowInteractiveShell(requestType string, command []string, hasPTY bool) bool {
	return requestType == "shell" && len(command) == 0 && hasPTY
}

// serve accepts SSH connections on listener until it closes.
func (s *server) serve(listener net.Listener) error {
	if s == nil || s.server == nil {
		return errors.New("SSH server is not configured")
	}
	return s.server.Serve(&observedListener{
		Listener:    listener,
		connections: s.connections,
		ready:       make(chan struct{}),
	})
}

// close immediately stops accepting connections and cancels active sessions.
func (s *server) close() error {
	if s == nil || s.server == nil {
		return nil
	}
	s.connections.closeAll()
	err := s.server.Close()
	s.connections.finishClose()
	return err
}

type observedListener struct {
	net.Listener
	connections *connectionTracker
	ready       chan struct{}
	once        sync.Once
}

func (l *observedListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	tracked, ok := l.connections.track(conn)
	if !ok {
		<-l.connections.closed
		return nil, net.ErrClosed
	}
	return tracked, nil
}

type connectionTracker struct {
	mu      sync.Mutex
	stopped bool
	conns   map[*trackedConn]struct{}
	closed  chan struct{}
	close   sync.Once
}

type trackedConn struct {
	net.Conn
	tracker *connectionTracker
	once    sync.Once
}

func newConnectionTracker() *connectionTracker {
	return &connectionTracker{conns: make(map[*trackedConn]struct{}), closed: make(chan struct{})}
}

func (t *connectionTracker) track(conn net.Conn) (*trackedConn, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		_ = conn.Close()
		return nil, false
	}
	tracked := &trackedConn{Conn: conn, tracker: t}
	t.conns[tracked] = struct{}{}
	return tracked, true
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() {
		c.tracker.mu.Lock()
		delete(c.tracker.conns, c)
		c.tracker.mu.Unlock()
	})
	return err
}

func (t *connectionTracker) closeAll() {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.stopped = true
	connections := make([]*trackedConn, 0, len(t.conns))
	for conn := range t.conns {
		connections = append(connections, conn)
	}
	t.mu.Unlock()
	for _, conn := range connections {
		_ = conn.Close()
	}
}

func (t *connectionTracker) finishClose() {
	t.close.Do(func() { close(t.closed) })
}

// Run serves SSH until ctx is canceled, then closes sessions before returning.
func Run(ctx context.Context, config Config, state *internal.State) error {
	if ctx == nil {
		return errors.New("SSH server context is nil")
	}
	server, err := newServer(config, state)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return fmt.Errorf("listen for SSH on %s: %w", config.Address, err)
	}
	ready := make(chan struct{})
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.serveWithReady(listener, ready) }()
	select {
	case <-ready:
	case err := <-serveResult:
		shutdownErr := stopServer(server)
		if errors.Is(err, charmssh.ErrServerClosed) {
			return shutdownErr
		}
		return errors.Join(err, shutdownErr)
	}

	select {
	case err := <-serveResult:
		shutdownErr := stopServer(server)
		if errors.Is(err, charmssh.ErrServerClosed) {
			return shutdownErr
		}
		return errors.Join(err, shutdownErr)
	case <-ctx.Done():
		shutdownErr := stopServer(server)
		serveErr := <-serveResult
		if shutdownErr != nil {
			return fmt.Errorf("shut down SSH server: %w", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, charmssh.ErrServerClosed) {
			return fmt.Errorf("serve SSH: %w", serveErr)
		}
		return nil
	}
}

func (s *server) serveWithReady(listener net.Listener, ready chan struct{}) error {
	return s.server.Serve(&observedListener{Listener: listener, connections: s.connections, ready: ready})
}

func stopServer(server *server) error {
	closeErr := server.close()
	waitCtx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
	defer cancel()
	shutdownErr := server.server.Shutdown(waitCtx)
	return errors.Join(closeErr, shutdownErr)
}
