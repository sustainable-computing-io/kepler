package dialers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

const (
	// defaultTLSPort specifies the default libvirtd port.
	defaultTLSPort = "16514"

	// defaultTLSTimeout specifies the default libvirt dial timeout.
	defaultTLSTimeout = 20 * time.Second
)

type certDirs struct {
	KeyPath  string
	CertPath string
}

// TLS implements connecting to a remote server's libvirt using tls
type TLS struct {
	timeout            time.Duration
	host, port         string
	insecureSkipVerify bool
	certSearchPaths    []certDirs
	caSearchPaths      []string
}

// TLSOption is a function for setting remote dialer options.
type TLSOption func(*TLS)

// WithInsecureNoVerify ignores the validity of the server certificate.
func WithInsecureNoVerify() TLSOption {
	return func(r *TLS) {
		r.insecureSkipVerify = true
	}
}

// UseTLSPort sets the port to dial for libirt on the target host server.
func UseTLSPort(port string) TLSOption {
	return func(r *TLS) {
		r.port = port
	}
}

// UsePKIPath sets the search path for TLS certificate files.
func UsePKIPath(pkiPath string) TLSOption {
	return func(r *TLS) {
		r.certSearchPaths = []certDirs{
			{
				KeyPath:  pkiPath,
				CertPath: pkiPath,
			},
		}
		r.caSearchPaths = []string{pkiPath}
	}
}

// NewTLS is a dialer for connecting to libvirt running on another server.
func NewTLS(hostAddr string, opts ...TLSOption) *TLS {
	r := &TLS{
		timeout: defaultTLSTimeout,
		host:    hostAddr,
		port:    defaultTLSPort,
		certSearchPaths: []certDirs{
			{
				KeyPath:  "/etc/pki/libvirt/private/",
				CertPath: "/etc/pki/libvirt/",
			},
		},
		caSearchPaths: []string{"/etc/pki/CA/"},
	}

	if u, err := user.Current(); err != nil || u.Uid != "0" {
		cd := filepath.Join(u.HomeDir, ".pki", "libvirt")
		r.certSearchPaths = append([]certDirs{{KeyPath: cd, CertPath: cd}}, r.certSearchPaths...)
		// Some libvirt docs erroneously state that the user location for the
		// CA cert is in ~/.pki/ but it is in fact in ~/.pki/libvirt/
		r.caSearchPaths = append([]string{cd}, r.caSearchPaths...)
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *TLS) clientCert() (*tls.Certificate, error) {
	var errs []error

	for _, dirs := range r.certSearchPaths {
		certFile, err := os.ReadFile(filepath.Join(dirs.CertPath, "clientcert.pem"))
		if err != nil {
			errs = append(errs,
				fmt.Errorf("could not read tls client cert: %w", err))
			continue
		}

		keyFile, err := os.ReadFile(filepath.Join(dirs.KeyPath, "clientkey.pem"))
		if err != nil {
			errs = append(errs,
				fmt.Errorf("could not read tls private key: %w", err))
			continue
		}

		cert, err := tls.X509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("invalid tls client cert: %w", err)
		}
		return &cert, nil
	}
	return nil, errors.Join(errs...)
}

func (r *TLS) caCerts(optional bool) (*x509.CertPool, error) {
	var errs []error
	pool := x509.NewCertPool()

	for _, dir := range r.caSearchPaths {
		if caFile, err := os.ReadFile(filepath.Join(dir, "cacert.pem")); err == nil {
			pool.AppendCertsFromPEM(caFile)
			return pool, nil
		} else if !(optional && errors.Is(err, fs.ErrNotExist)) {
			errs = append(errs,
				fmt.Errorf("could not read tls CA cert: %w", err))
		}
	}
	return nil, errors.Join(errs...)
}

func (r *TLS) config() (*tls.Config, error) {
	cert, err := r.clientCert()
	if err != nil {
		return nil, err
	}
	rootCAs, err := r.caCerts(r.insecureSkipVerify)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{*cert},
		RootCAs:            rootCAs,
		InsecureSkipVerify: r.insecureSkipVerify, //nolint:gosec
	}, nil
}

// Dial connects to libvirt running on another server.
func (r *TLS) Dial() (net.Conn, error) {
	conf, err := r.config()
	if err != nil {
		return nil, err
	}

	netDialer := net.Dialer{
		Timeout: r.timeout,
	}
	c, err := tls.DialWithDialer(
		&netDialer,
		"tcp",
		net.JoinHostPort(r.host, r.port),
		conf,
	)
	if err != nil {
		return nil, err
	}

	// When running over TLS, after connection libvirt writes a single byte to
	// the socket to indicate whether the server's check of the client's
	// certificate has succeeded.
	// See https://github.com/digitalocean/go-libvirt/issues/89#issuecomment-1607300636
	// for more details.
	buf := make([]byte, 1)
	if n, err := c.Read(buf); err != nil {
		c.Close()
		return nil, err
	} else if n != 1 || buf[0] != byte(1) {
		c.Close()
		return nil, errors.New("server verification (of our certificate or IP address) failed")
	}

	return c, nil
}
