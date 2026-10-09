package ssh

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func TestLoadConfigDefaultsAndEnvironment(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want Config
	}{
		{
			name: "defaults",
			want: Config{
				Address:        "127.0.0.1:23234",
				AuthorizedKeys: filepath.Join("/home", "reader", ".ssh", "authorized_keys"),
				HostKey:        filepath.Join("/home", "reader", ".local", "share", "atom1c", "ssh_host_ed25519_key"),
				Username:       "reader",
			},
		},
		{
			name: "environment overrides and XDG data home",
			env: map[string]string{
				"ATOM1C_SSH_ADDR":        "0.0.0.0:2222",
				"ATOM1C_AUTHORIZED_KEYS": "/config/keys",
				"ATOM1C_SSH_HOST_KEY":    "/data/host_key",
				"XDG_DATA_HOME":          "/xdg",
			},
			want: Config{
				Address:        "0.0.0.0:2222",
				AuthorizedKeys: "/config/keys",
				HostKey:        "/data/host_key",
				Username:       "reader",
			},
		},
		{
			name: "XDG controls default host key path",
			env:  map[string]string{"XDG_DATA_HOME": "/xdg"},
			want: Config{
				Address:        "127.0.0.1:23234",
				AuthorizedKeys: filepath.Join("/home", "reader", ".ssh", "authorized_keys"),
				HostKey:        filepath.Join("/xdg", "atom1c", "ssh_host_ed25519_key"),
				Username:       "reader",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := LoadConfig(func(key string) string { return test.env[key] }, "/home/reader", "reader")
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("LoadConfig() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidListenAddress(t *testing.T) {
	_, err := LoadConfig(func(key string) string {
		if key == "ATOM1C_SSH_ADDR" {
			return "not-an-address"
		}
		return ""
	}, "/home/reader", "reader")
	if err == nil {
		t.Fatal("LoadConfig() accepted an invalid listen address")
	}
}

func TestParseAuthorizedKeys(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := gossh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	_, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caSigner, err := gossh.NewSignerFromKey(caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &gossh.Certificate{
		Key:             publicKey,
		CertType:        gossh.UserCert,
		ValidPrincipals: []string{"reader"},
		ValidAfter:      0,
		ValidBefore:     gossh.CertTimeInfinity,
	}
	if err := certificate.SignCert(rand.Reader, caSigner); err != nil {
		t.Fatal(err)
	}
	line := string(gossh.MarshalAuthorizedKey(publicKey))
	tests := []struct {
		name    string
		content string
		want    int
		wantErr string
	}{
		{name: "comments and blank lines", content: "# owner key\n\n" + line, want: 1},
		{name: "empty file", content: "# no keys\n", wantErr: "no authorized keys"},
		{name: "unsupported key options", content: "command=\"true\" " + strings.TrimSpace(line) + "\n", wantErr: "options are not supported"},
		{name: "certificate entry", content: string(gossh.MarshalAuthorizedKey(certificate)), wantErr: "certificates are not supported"},
		{name: "malformed key", content: "ssh-ed25519 not-base64 key\n", wantErr: "parse authorized key"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keys, err := parseAuthorizedKeys(strings.NewReader(test.content))
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parseAuthorizedKeys() error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAuthorizedKeys() error = %v", err)
			}
			if len(keys) != test.want {
				t.Fatalf("parsed %d keys, want %d", len(keys), test.want)
			}
		})
	}
}

func TestLoadHostSignerCreatesAndReusesPrivateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "host_key")
	signer, err := loadHostSigner(path)
	if err != nil {
		t.Fatalf("loadHostSigner() = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat generated key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("host key permissions = %o, want 600", info.Mode().Perm())
	}
	if dirInfo, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("stat host-key directory: %v", err)
	} else if dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("host-key directory permissions = %o, want 700", dirInfo.Mode().Perm())
	}
	second, err := loadHostSigner(path)
	if err != nil {
		t.Fatalf("loadHostSigner() on restart = %v", err)
	}
	if !bytes.Equal(signer.PublicKey().Marshal(), second.PublicKey().Marshal()) {
		t.Fatal("host public key changed after reload")
	}
}

func TestLoadHostSignerDoesNotReplaceInvalidExistingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_key")
	contents := []byte("not a private key")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadHostSigner(path); err == nil {
		t.Fatal("loadHostSigner() accepted invalid existing key")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatal("loadHostSigner() modified invalid existing key")
	}
}

func TestLoadHostSignerRejectsNonEd25519ExistingKey(t *testing.T) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := gossh.MarshalPrivateKey(private, "test")
	if err != nil {
		t.Fatal(err)
	}
	block := pem.EncodeToMemory(der)
	path := filepath.Join(t.TempDir(), "host_key")
	if err := os.WriteFile(path, block, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadHostSigner(path); err == nil || !strings.Contains(err.Error(), "must be Ed25519") {
		t.Fatalf("loadHostSigner() error = %v, want non-Ed25519 rejection", err)
	}
}

func TestAuthorizedKeySetAllowsOnlyExpectedUsernameAndKey(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := gossh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	otherPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPublicKey, err := gossh.NewPublicKey(otherPublic)
	if err != nil {
		t.Fatal(err)
	}
	keys := authorizedKeySet{string(publicKey.Marshal()): publicKey}
	tests := []struct {
		username string
		key      gossh.PublicKey
		want     bool
	}{
		{username: "reader", key: publicKey, want: true},
		{username: "reader", key: otherPublicKey},
		{username: "other", key: publicKey},
		{username: "reader", key: nil},
	}
	for _, test := range tests {
		if got := keys.Allows("reader", test.username, test.key); got != test.want {
			t.Errorf("Allows(%q, key) = %v, want %v", test.username, got, test.want)
		}
	}
}

func TestLoadAuthorizedKeyFileReportsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	_, err := loadAuthorizedKeyFile(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("loadAuthorizedKeyFile() error = %v, want path %q", err, path)
	}
}
