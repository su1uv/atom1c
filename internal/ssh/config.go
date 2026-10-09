package ssh

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gossh "golang.org/x/crypto/ssh"
)

const (
	defaultListenAddress = "127.0.0.1:23234"
	hostKeyFileName      = "ssh_host_ed25519_key"
)

// Config describes the SSH server identity and local key-file inputs.
type Config struct {
	Address        string
	AuthorizedKeys string
	HostKey        string
	Username       string
}

// LoadConfig builds server configuration from the provided environment lookup,
// home directory, and operating-system username.
func LoadConfig(getenv func(string) string, home, username string) (Config, error) {
	if getenv == nil {
		return Config{}, errors.New("SSH environment lookup is nil")
	}
	if home == "" || !filepath.IsAbs(home) {
		return Config{}, fmt.Errorf("home directory must be an absolute path")
	}
	if strings.TrimSpace(username) == "" {
		return Config{}, errors.New("operating-system username is empty")
	}

	address := envOr(getenv, "ATOM1C_SSH_ADDR", defaultListenAddress)
	if err := validateListenAddress(address); err != nil {
		return Config{}, fmt.Errorf("invalid ATOM1C_SSH_ADDR: %w", err)
	}
	authorizedKeys := envOr(getenv, "ATOM1C_AUTHORIZED_KEYS", filepath.Join(home, ".ssh", "authorized_keys"))
	hostKey := getenv("ATOM1C_SSH_HOST_KEY")
	if hostKey == "" {
		dataHome := getenv("XDG_DATA_HOME")
		if dataHome == "" || !filepath.IsAbs(dataHome) {
			dataHome = filepath.Join(home, ".local", "share")
		}
		hostKey = filepath.Join(dataHome, "atom1c", hostKeyFileName)
	}
	return Config{
		Address:        address,
		AuthorizedKeys: authorizedKeys,
		HostKey:        hostKey,
		Username:       username,
	}, nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if value := getenv(key); value != "" {
		return value
	}
	return fallback
}

func validateListenAddress(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if strings.TrimSpace(host) == "" {
		return errors.New("host must not be empty")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("port %q must be between 1 and 65535", port)
	}
	return nil
}

// parseAuthorizedKeys loads plain public-key lines. OpenSSH key restrictions
// and certificates are rejected because this server does not enforce them.
func parseAuthorizedKeys(reader io.Reader) (authorizedKeySet, error) {
	if reader == nil {
		return nil, errors.New("authorized keys reader is nil")
	}
	keys := make(authorizedKeySet)
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, options, _, err := gossh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("parse authorized key at line %d: %w", lineNumber, err)
		}
		if _, isCertificate := key.(*gossh.Certificate); isCertificate {
			return nil, fmt.Errorf("authorized key certificates are not supported (line %d)", lineNumber)
		}
		if len(options) != 0 {
			return nil, fmt.Errorf("authorized key options are not supported (line %d)", lineNumber)
		}
		keys[string(key.Marshal())] = key
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read authorized keys: %w", err)
	}
	if len(keys) == 0 {
		return nil, errors.New("authorized keys file contains no authorized keys")
	}
	return keys, nil
}

type authorizedKeySet map[string]gossh.PublicKey

func (keys authorizedKeySet) Allows(owner, username string, key gossh.PublicKey) bool {
	if owner == "" || username != owner || key == nil {
		return false
	}
	_, ok := keys[string(key.Marshal())]
	return ok
}

// loadAuthorizedKeyFile reads and validates the complete authorized_keys file.
func loadAuthorizedKeyFile(path string) (authorizedKeySet, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open authorized keys %q: %w", path, err)
	}
	defer file.Close()
	keys, err := parseAuthorizedKeys(file)
	if err != nil {
		return nil, fmt.Errorf("authorized keys %q: %w", path, err)
	}
	return keys, nil
}

// loadHostSigner loads the persistent Ed25519 host key or creates it with
// owner-only permissions when the configured path does not exist.
func loadHostSigner(path string) (gossh.Signer, error) {
	if path == "" {
		return nil, errors.New("host-key path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create host-key directory: %w", err)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := createHostKey(path); err != nil {
			return nil, fmt.Errorf("create host key %q: %w", path, err)
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, fmt.Errorf("stat host key %q: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("host key %q must be a regular file", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("host key %q must not be accessible by group or others (permissions %o)", path, info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read host key %q: %w", path, err)
	}
	signer, err := gossh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse host key %q: %w", path, err)
	}
	if signer.PublicKey().Type() != gossh.KeyAlgoED25519 {
		return nil, fmt.Errorf("host key %q must be Ed25519", path)
	}
	return signer, nil
}

func createHostKey(path string) ([]byte, error) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 host key: %w", err)
	}
	block, err := gossh.MarshalPrivateKey(privateKey, "atom1c SSH host key")
	if err != nil {
		return nil, fmt.Errorf("encode Ed25519 host key: %w", err)
	}
	data := pem.EncodeToMemory(block)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create host key %q: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write host key %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("sync host key %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close host key %q: %w", path, err)
	}
	return data, nil
}
