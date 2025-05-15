// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	serverCertFile = "cert.pem"
	serverKeyFile  = "key.pem"
	clientCertFile = "client-cert.pem"
	clientKeyFile  = "client-key.pem"
	caCertFile     = "ca-cert.pem"

	serverCertPath = "testdata/cert.pem"
	serverKeyPath  = "testdata/key.pem"
	clientCertPath = "testdata/client-cert.pem"
	clientKeyPath  = "testdata/client-key.pem"
	caCertPath     = "testdata/ca-cert.pem"
)

// GenerateTestCerts generates a self-signed server certificate and key using openssl.
func GenerateTestCerts(t *testing.T) (string, string) {
	t.Helper()

	if err := os.MkdirAll("testdata", 0755); err != nil {
		t.Fatalf("Failed to create testdata directory: %v", err)
	}

	// Generate CA private key
	caKey := "testdata/ca-key.pem"
	cmd := exec.Command("openssl", "genrsa", "-out", caKey, "2048")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate CA key: %v\nOutput: %s", err, output)
	}

	// Generate CA certificate
	caConf := `
[req]
distinguished_name = req_dn
x509_extensions = v3_ca
prompt = no

[req_dn]
CN = Test CA

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
basicConstraints = critical, CA:true
`
	caConfFile := "testdata/ca.conf"
	if err := os.WriteFile(caConfFile, []byte(caConf), 0644); err != nil {
		t.Fatalf("Failed to write CA config: %v", err)
	}
	cmd = exec.Command("openssl", "req", "-x509", "-new", "-nodes",
		"-key", caKey, "-days", "1", "-out", caCertPath,
		"-config", caConfFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate CA cert: %v\nOutput: %s", err, output)
	}

	// Generate server private key
	cmd = exec.Command("openssl", "genrsa", "-out", serverKeyPath, "2048")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate server key: %v\nOutput: %s", err, output)
	}

	// Generate server CSR with SANs
	serverCsr := "testdata/server.csr"
	serverConf := `
[req]
distinguished_name = req_dn
req_extensions = v3_req
prompt = no

[req_dn]
CN = localhost

[v3_req]
subjectAltName = DNS:localhost,IP:127.0.0.1
`
	serverConfFile := "testdata/server.conf"
	if err := os.WriteFile(serverConfFile, []byte(serverConf), 0644); err != nil {
		t.Fatalf("Failed to write server config: %v", err)
	}
	cmd = exec.Command("openssl", "req", "-new",
		"-key", serverKeyPath, "-out", serverCsr,
		"-config", serverConfFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate server CSR: %v\nOutput: %s", err, output)
	}

	// Sign server certificate with CA
	cmd = exec.Command("openssl", "x509", "-req",
		"-in", serverCsr, "-CA", caCertPath, "-CAkey", caKey,
		"-CAcreateserial", "-out", serverCertPath, "-days", "1",
		"-extfile", serverConfFile, "-extensions", "v3_req")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to sign server cert: %v\nOutput: %s", err, output)
	}

	return serverCertFile, serverKeyFile
}

// GenerateClientCerts generates a client certificate and key signed by the same CA.
func GenerateClientCerts(t *testing.T) (string, string) {
	t.Helper()

	// Generate client private key
	if err := exec.Command("openssl", "genrsa", "-out", clientKeyPath, "2048").Run(); err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	// Generate client CSR
	clientCsr := "testdata/client.csr"
	clientConf := `
[req]
distinguished_name = req_dn
prompt = no

[req_dn]
CN = Test Client
`
	clientConfFile := "testdata/client.conf"
	if err := os.WriteFile(clientConfFile, []byte(clientConf), 0644); err != nil {
		t.Fatalf("Failed to write client config: %v", err)
	}
	if err := exec.Command("openssl", "req", "-new",
		"-key", clientKeyPath, "-out", clientCsr,
		"-config", clientConfFile).Run(); err != nil {
		t.Fatalf("Failed to generate client CSR: %v", err)
	}

	// Sign client certificate with CA
	if err := exec.Command("openssl", "x509", "-req",
		"-in", clientCsr, "-CA", caCertPath, "-CAkey", "testdata/ca-key.pem",
		"-CAcreateserial", "-out", clientCertPath, "-days", "1").Run(); err != nil {
		t.Fatalf("Failed to sign client cert: %v", err)
	}

	return clientCertPath, clientKeyPath
}

// httpsClient creates an HTTP client with TLS.
func httpsClient(t *testing.T, caCertPath, clientCertPath, clientKeyPath string, skipVerify bool) *http.Client {
	t.Helper()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			t.Fatalf("Failed to read CA cert: %v", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			t.Fatalf("Failed to append CA cert to pool")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if clientCertPath != "" && clientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			t.Fatalf("Failed to load client cert/key: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 5 * time.Second,
	}
}

// writeWebConfig writes a web.yml file with TLS config
func writeWebConfig(t *testing.T, config string) string {
	t.Helper()

	webConfigFile := "testdata/web.yml"
	if err := os.WriteFile(webConfigFile, []byte(config), 0644); err != nil {
		t.Fatalf("Failed to write web config: %v", err)
	}
	return webConfigFile
}

func TestTLSConfigWithWebConfigFile(t *testing.T) {
	// Generate server certificates
	serverCert, serverKey := GenerateTestCerts(t)

	// Cleanup testdata directory after tests
	t.Cleanup(func() {
		if err := os.RemoveAll("testdata"); err != nil {
			t.Logf("Failed to clean up testdata: %v", err)
		}
	})

	t.Run("Basic TLS", func(t *testing.T) {
		webConfig := fmt.Sprintf(`
tls_server_config:
  cert_file: %s
  key_file: %s
`, serverCert, serverKey)
		webConfigFile := writeWebConfig(t, webConfig)

		port := findFreePort()

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		server := NewAPIServer(
			WithListenAddress([]string{addr}),
			WithWebConfig(webConfigFile))
		assert.NoError(t, server.Init())

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := server.Register("/api/test", "Test API", "Test API endpoint", testHandler)
		require.NoError(t, err)

		errCh := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		go func() {
			errCh <- server.Run(ctx)
		}()

		time.Sleep(300 * time.Millisecond)

		client := httpsClient(t, caCertPath, "", "", false)
		resp, err := client.Get(fmt.Sprintf("https://%s/", addr))
		require.NoError(t, err, "HTTP request to root endpoint failed")
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

		// Wait for server to stop after context timeout
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Server didn't shut down within expected timeframe")
		}
	})

	// Sub-test: Mutual TLS with valid client cert
	t.Run("MutualTLS", func(t *testing.T) {
		// Generate client certificates
		clientCert, clientKey := GenerateClientCerts(t)

		webConfig := fmt.Sprintf(`
tls_server_config:
  cert_file: %s
  key_file: %s
  client_auth_type: RequireAndVerifyClientCert
  client_ca_file: %s
`, serverCert, serverKey, caCertFile)
		webConfigFile := writeWebConfig(t, webConfig)

		port := findFreePort()

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		server := NewAPIServer(
			WithListenAddress([]string{addr}),
			WithWebConfig(webConfigFile))
		assert.NoError(t, server.Init())

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := server.Register("/api/test", "Test API", "Test API endpoint", testHandler)
		require.NoError(t, err)

		errCh := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		go func() {
			errCh <- server.Run(ctx)
		}()

		time.Sleep(300 * time.Millisecond)

		client := httpsClient(t, caCertPath, clientCert, clientKey, false)
		resp, err := client.Get(fmt.Sprintf("https://%s/", addr))
		require.NoError(t, err, "HTTP request to root endpoint failed")
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))

		// Wait for server to stop after context timeout
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Server didn't shut down within expected timeframe")
		}
	})

	// Sub-test: Mutual TLS without client cert
	t.Run("MutualTLSNoClientCert", func(t *testing.T) {
		webConfig := fmt.Sprintf(`
tls_server_config:
  cert_file: %s
  key_file: %s
  client_auth_type: RequireAndVerifyClientCert
  client_ca_file: %s
`, serverCert, serverKey, caCertFile)
		webConfigFile := writeWebConfig(t, webConfig)

		port := findFreePort()

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		server := NewAPIServer(
			WithListenAddress([]string{addr}),
			WithWebConfig(webConfigFile))
		assert.NoError(t, server.Init())

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		err := server.Register("/api/test", "Test API", "Test API endpoint", testHandler)
		require.NoError(t, err)

		errCh := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		go func() {
			errCh <- server.Run(ctx)
		}()

		time.Sleep(300 * time.Millisecond)

		// Client without client cert
		client := httpsClient(t, caCertPath, "", "", false)
		_, err = client.Get(fmt.Sprintf("https://%s/", addr))
		if err == nil {
			t.Error("Expected request to fail due to missing client cert")
		} else if !strings.Contains(err.Error(), "tls") {
			t.Errorf("Expected TLS error, got: %v", err)
		}

		// Wait for server to stop after context timeout
		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("Server didn't shut down within expected timeframe")
		}
	})

	// Sub-test: Invalid web.yml configuration
	t.Run("InvalidWebConfig", func(t *testing.T) {
		webConfig := `
tls_server_config:
  cert_file: /nonexistent/cert.pem
  key_file: /nonexistent/key.pem
`
		webConfigFile := writeWebConfig(t, webConfig)

		port := findFreePort()

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		server := NewAPIServer(
			WithListenAddress([]string{addr}),
			WithWebConfig(webConfigFile))
		assert.NoError(t, server.Init())

		errCh := make(chan error, 1)
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		errCh <- server.Run(ctx)
		err := <-errCh

		if err == nil {
			t.Error("Expected server to fail due to invalid config")
		} else if !strings.Contains(err.Error(), "failed to read") {
			t.Errorf("Expected file load error, got: %v", err)
		}
	})
}
