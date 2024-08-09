package libvirt

import (
	"errors"
	"fmt"
	"net/url"
	"os/user"
	"strconv"
	"strings"

	"github.com/digitalocean/go-libvirt/socket"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

// ConnectToURI returns a new, connected client instance using the appropriate
// dialer for the given libvirt URI.
func ConnectToURI(uri *url.URL) (*Libvirt, error) {
	dialer, err := dialerForURI(uri)
	if err != nil {
		return nil, err
	}

	lv := NewWithDialer(dialer)

	if err := lv.ConnectToURI(RemoteURI(uri)); err != nil {
		return nil, fmt.Errorf("failed to connect to libvirt: %w", err)
	}

	return lv, nil
}

// RemoteURI returns the libvirtd URI corresponding to a given client URI.
// The client URI contains details of the connection method, but once connected
// to libvirtd, all connections are local. So e.g. the client may want to
// connect to qemu+tcp://example.com/system but once the socket is established
// it will ask the remote libvirtd for qemu:///system.
func RemoteURI(uri *url.URL) ConnectURI {
	remoteURI := (&url.URL{
		Scheme: strings.Split(uri.Scheme, "+")[0],
		Path:   uri.Path,
	}).String()
	if name := uri.Query().Get("name"); name != "" {
		remoteURI = name
	}
	return ConnectURI(remoteURI)
}

func dialerForURI(uri *url.URL) (socket.Dialer, error) {
	transport := "unix"
	if scheme := strings.SplitN(uri.Scheme, "+", 2); len(scheme) > 1 {
		transport = scheme[1]
	} else if uri.Host != "" {
		transport = "tls"
	}

	switch transport {
	case "unix":
		options := []dialers.LocalOption{}
		if s := uri.Query().Get("socket"); s != "" {
			options = append(options, dialers.WithSocket(s))
		}
		if err := checkModeOption(uri); err != nil {
			return nil, err
		}
		return dialers.NewLocal(options...), nil
	case "tcp":
		options := []dialers.RemoteOption{}
		if port := uri.Port(); port != "" {
			options = append(options, dialers.UsePort(port))
		}
		return dialers.NewRemote(uri.Hostname(), options...), nil
	case "tls":
		options := []dialers.TLSOption{}
		if port := uri.Port(); port != "" {
			options = append(options, dialers.UseTLSPort(port))
		}
		if pkiPath := uri.Query().Get("pkipath"); pkiPath != "" {
			options = append(options, dialers.UsePKIPath(pkiPath))
		}
		if nv, err := noVerifyOption(uri); err != nil {
			return nil, err
		} else if nv {
			options = append(options, dialers.WithInsecureNoVerify())
		}
		return dialers.NewTLS(uri.Hostname(), options...), nil
	case "libssh", "libssh2":
		options := []dialers.SSHOption{}
		options, err := processCommonSSHOptions(uri, options)
		if err != nil {
			return nil, err
		}
		if knownHosts := uri.Query().Get("known_hosts"); knownHosts != "" {
			options = append(options, dialers.UseKnownHostsFile(knownHosts))
		}
		if hostVerify := uri.Query().Get("known_hosts_verify"); hostVerify != "" {
			switch hostVerify {
			case "normal":
			case "auto":
				options = append(options, dialers.WithAcceptUnknownHostKey())
			case "ignore":
				options = append(options, dialers.WithInsecureIgnoreHostKey())
			default:
				return nil, fmt.Errorf("invalid ssh known hosts verify method %v", hostVerify)
			}
		}
		if auth := uri.Query().Get("sshauth"); auth != "" {
			authMethods := &dialers.SSHAuthMethods{}
			for _, a := range strings.Split(auth, ",") {
				switch strings.ToLower(a) {
				case "agent":
					authMethods.Agent()
				case "privkey":
					authMethods.PrivKey()
				case "password":
					authMethods.Password()
				case "keyboard-interactive":
					authMethods.KeyboardInteractive()
				default:
					return nil, fmt.Errorf("invalid ssh auth method %v", a)
				}
			}
			options = append(options, dialers.WithSSHAuthMethods(authMethods))
		}
		if noVerify := uri.Query().Get("no_verify"); noVerify != "" {
			return nil, fmt.Errorf(
				"\"no_verify\" option invalid with %s transport, use known_hosts_verify=ignore instead",
				transport)
		}
		return dialers.NewSSH(uri.Hostname(), options...), nil
	case "ssh":
		// Emulate ssh using golang ssh library. Note that this means that
		// system ssh config is not respected as it would be when shelling out
		// to the ssh binary.
		currentUser, err := user.Current()
		if err != nil {
			return nil, err
		}
		options := []dialers.SSHOption{
			dialers.WithSystemSSHDefaults(currentUser),
		}
		options, err = processCommonSSHOptions(uri, options)
		if err != nil {
			return nil, err
		}
		if nv, err := noVerifyOption(uri); err != nil {
			return nil, err
		} else if nv {
			options = append(options, dialers.WithInsecureIgnoreHostKey())
		}

		fieldErrs := []error{}
		for _, f := range []string{
			"known_hosts",
			"known_hosts_verify",
			"sshauth",
		} {
			if field := uri.Query().Get(f); field != "" {
				fieldErrs = append(fieldErrs,
					fmt.Errorf("%v option invalid with ssh transport, use libssh transport instead", f))
			}
		}
		if len(fieldErrs) > 0 {
			return nil, errors.Join(fieldErrs...)
		}

		return dialers.NewSSH(uri.Hostname(), options...), nil
	default:
		return nil, fmt.Errorf("unsupported libvirt transport %s", transport)
	}
}

func noVerifyOption(uri *url.URL) (bool, error) {
	nv := uri.Query().Get("no_verify")
	if nv == "" {
		return false, nil
	}
	val, err := strconv.Atoi(nv)
	if err != nil {
		return false, fmt.Errorf("invalid value for no_verify: %w", err)
	}
	return val != 0, nil
}

func checkModeOption(uri *url.URL) error {
	mode := uri.Query().Get("mode")
	switch strings.ToLower(mode) {
	case "":
	case "legacy", "auto":
	case "direct":
		return errors.New("cannot connect in direct mode")
	default:
		return fmt.Errorf("invalid ssh mode %v", mode)
	}
	return nil
}

func processCommonSSHOptions(uri *url.URL, options []dialers.SSHOption) ([]dialers.SSHOption, error) {
	if port := uri.Port(); port != "" {
		options = append(options, dialers.UseSSHPort(port))
	}
	if username := uri.User.Username(); username != "" {
		options = append(options, dialers.UseSSHUsername(username))
	}
	if password, ok := uri.User.Password(); ok {
		options = append(options, dialers.UseSSHPassword(password))
	}
	if socket := uri.Query().Get("socket"); socket != "" {
		options = append(options, dialers.WithRemoteSocket(socket))
	}
	if keyFile := uri.Query().Get("keyfile"); keyFile != "" {
		options = append(options, dialers.UseKeyFile(keyFile))
	}
	if err := checkModeOption(uri); err != nil {
		return options, err
	}
	return options, nil
}
