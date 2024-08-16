package dialers

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	// defaultSSHPort specifies the default ssh port.
	defaultSSHPort = "22"

	// defaultSSHTimeout specified the default ssh dial timeout.
	defaultSSHTimeout = 20 * time.Second
)

// SSHAuthMethods maintains a priority list of allowed ssh auth methods.
type SSHAuthMethods struct {
	authMethodGenerators []func(s *SSH) ssh.AuthMethod
	signers              []ssh.Signer
	errors               []error
}

// SSH implements connecting to a remote server's libvirt using ssh.
type SSH struct {
	dialTimeout           time.Duration
	hostname, port        string
	username, password    string
	insecureIgnoreHostKey bool
	acceptUnknownHostKey  bool
	keyFile               string
	knownHostsFile        string
	authMethods           *SSHAuthMethods
	remoteSocket          string
}

// SSHOption is a function for setting ssh dialer options.
type SSHOption func(*SSH)

// UseSSHUsername uses the given username for the ssh connection.
func UseSSHUsername(username string) SSHOption {
	return func(s *SSH) {
		if username != "" {
			s.username = username
		}
	}
}

// UseSSHPassword uses the given password for the ssh connection.
func UseSSHPassword(password string) SSHOption {
	return func(s *SSH) {
		if password != "" {
			s.password = password
		}
	}
}

// UseSSHPort uses the given port for the ssh connection.
func UseSSHPort(port string) SSHOption {
	return func(s *SSH) {
		if port != "" {
			s.port = port
		}
	}
}

// WithAcceptUnknownHostKey ignores the validity of the host certificate.
func WithAcceptUnknownHostKey() SSHOption {
	return func(s *SSH) {
		s.acceptUnknownHostKey = true
	}
}

// WithInsecureIgnoreHostKey ignores the validity of the host certificate.
func WithInsecureIgnoreHostKey() SSHOption {
	return func(s *SSH) {
		s.insecureIgnoreHostKey = true
	}
}

// UseKnownHostsFile uses a custom known_hosts file
func UseKnownHostsFile(filename string) SSHOption {
	return func(s *SSH) {
		s.knownHostsFile = filename
	}
}

// UseKeyFile uses a custom key file
func UseKeyFile(filename string) SSHOption {
	return func(s *SSH) {
		s.keyFile = filename
	}
}

// WithSSHAuthMethods uses the specified auth methods in priority order.
func WithSSHAuthMethods(methods *SSHAuthMethods) SSHOption {
	return func(s *SSH) {
		s.authMethods = methods
	}
}

// WithRemoteSocket uses a custom remote socket
func WithRemoteSocket(socket string) SSHOption {
	return func(s *SSH) {
		s.remoteSocket = socket
	}
}

// WithSystemSSHDefaults uses default values for the system ssh client,
// rather than the defaults that libvirtclient uses with libssh.
func WithSystemSSHDefaults(currentUser *user.User) SSHOption {
	return UseKnownHostsFile(filepath.Join(currentUser.HomeDir,
		".ssh",
		"known_hosts"))
}

func defaultSSHKeyFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	for _, key := range []string{
		"identity",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		"id_rsa",
	} {
		path := filepath.Join(homeDir, ".ssh", key)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func defaultSSHKnownHostsFile() string {
	configDir, err := os.UserConfigDir()
	if err == nil {
		if runtime.GOOS != "windows" {
			configDir = filepath.Join(configDir, "libvirt")
		}
		return filepath.Join(configDir, "known_hosts")
	}
	return ""
}

// PrivKey adds the Private Key auth method to the allowed list.
func (am *SSHAuthMethods) PrivKey() *SSHAuthMethods {
	am.authMethodGenerators = append(am.authMethodGenerators,
		func(s *SSH) ssh.AuthMethod {
			key, err := os.ReadFile(s.keyFile)
			if err != nil {
				am.errors = append(am.errors,
					fmt.Errorf("failed to read ssh key %v: %w", s.keyFile, err))
				return nil
			}

			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				am.errors = append(am.errors,
					fmt.Errorf("failed to parse ssh key %v: %w", s.keyFile, err))
				return nil
			}
			// Only one callback of type "publickey" can be used, so the
			// privkey and agent methods must share a callback.
			first := len(am.signers) == 0
			am.signers = append(am.signers, signer)
			if first {
				return ssh.PublicKeysCallback(am.getSigners)
			}
			return nil
		})
	return am
}

// Agent adds the ssh agent auth method to the allowed list.
func (am *SSHAuthMethods) Agent() *SSHAuthMethods {
	am.authMethodGenerators = append(am.authMethodGenerators,
		func(s *SSH) ssh.AuthMethod {
			socket := os.Getenv("SSH_AUTH_SOCK")
			if socket == "" {
				return nil
			}

			conn, err := net.Dial("unix", socket)
			if err != nil {
				am.errors = append(am.errors,
					fmt.Errorf("failed to connect to agent socket %v: %w", socket, err))
				return nil
			}

			agentClient := agent.NewClient(conn)
			if signers, err := agentClient.Signers(); err != nil {
				am.errors = append(am.errors,
					fmt.Errorf("failed to get signers from agent: %w", err))
			} else {
				// Only one callback of type "publickey" can be used, so the
				// privkey and agent methods must share a callback.
				first := len(am.signers) == 0
				am.signers = append(am.signers, signers...)
				if first && len(am.signers) > 0 {
					return ssh.PublicKeysCallback(am.getSigners)
				}
			}
			return nil
		})
	return am
}

// Password adds the password auth method to the allowed list.
func (am *SSHAuthMethods) Password() *SSHAuthMethods {
	am.authMethodGenerators = append(am.authMethodGenerators,
		func(s *SSH) ssh.AuthMethod {
			if s.password == "" {
				am.errors = append(am.errors,
					errors.New("no ssh password set"))
				return nil
			}
			return ssh.Password(s.password)
		})
	return am
}

// KeyboardInteractive adds the keyboard-interactive auth method to the
// allowed list (currently unimplemented).
func (am *SSHAuthMethods) KeyboardInteractive() *SSHAuthMethods {
	// Not implemented
	return am
}

func (am *SSHAuthMethods) getSigners() ([]ssh.Signer, error) {
	return am.signers, nil
}

func (am *SSHAuthMethods) authMethods(s *SSH) []ssh.AuthMethod {
	am.signers = nil
	am.errors = nil
	methods := []ssh.AuthMethod{}
	for _, g := range am.authMethodGenerators {
		if m := g(s); m != nil {
			methods = append(methods, m)
		}
	}
	return methods
}

// NewSSH returns an ssh dialer for connecting to libvirt running on another
// server.
func NewSSH(hostAddr string, opts ...SSHOption) *SSH {
	defaultUsername := ""
	if currentUser, err := user.Current(); err == nil {
		defaultUsername = currentUser.Username
	}

	s := &SSH{
		dialTimeout:    defaultSSHTimeout,
		username:       defaultUsername,
		hostname:       hostAddr,
		port:           defaultSSHPort,
		remoteSocket:   defaultSocket,
		knownHostsFile: defaultSSHKnownHostsFile(),
		keyFile:        defaultSSHKeyFile(),
		authMethods:    (&SSHAuthMethods{}).Agent().PrivKey().Password().KeyboardInteractive(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func appendKnownHost(knownHostsFile string, host string, key ssh.PublicKey) {
	if err := os.MkdirAll(filepath.Dir(knownHostsFile), 0700); err != nil {
		return
	}

	f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "%s\n", knownhosts.Line([]string{host}, key))
}

func (s *SSH) checkHostKey(host string, remote net.Addr, key ssh.PublicKey) error {
	checkKnown, err := knownhosts.New(s.knownHostsFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && s.acceptUnknownHostKey {
			appendKnownHost(s.knownHostsFile, host, key)
			return nil
		}
		return err
	}

	result := checkKnown(host, remote, key)
	if keyErr, ok := result.(*knownhosts.KeyError); ok {
		if len(keyErr.Want) == 0 && s.acceptUnknownHostKey {
			appendKnownHost(s.knownHostsFile, host, key)
			return nil
		}
	}
	return result
}

func (s *SSH) config() (*ssh.ClientConfig, error) {
	hostKeyCallback := s.checkHostKey
	if s.insecureIgnoreHostKey {
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	}
	return &ssh.ClientConfig{
		User:            s.username,
		HostKeyCallback: hostKeyCallback,
		Auth:            s.authMethods.authMethods(s),
		Timeout:         s.dialTimeout,
	}, nil
}

// Dial connects to libvirt running on another server over ssh.
func (s *SSH) Dial() (net.Conn, error) {
	conf, err := s.config()
	if err != nil {
		return nil, err
	}

	sshClient, err := ssh.Dial("tcp", net.JoinHostPort(s.hostname, s.port),
		conf)
	if err != nil {
		if strings.HasPrefix(err.Error(), "ssh: handshake failed: ssh: unable to authenticate") {
			err = errors.Join(append([]error{err}, s.authMethods.errors...)...)
		}
		return nil, err
	}
	c, err := sshClient.Dial("unix", s.remoteSocket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote libvirt socket %s: %w", s.remoteSocket, err)
	}

	return c, nil
}
