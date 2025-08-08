// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/stmcginnis/gofish"
)

// gofishClient wraps the Gofish Redfish client with connection management
type (
	GoFishClient interface {
		Connect(context.Context) error
		Disconnect()
		IsConnected() bool
		GetAPIClient() *gofish.APIClient
		Endpoint() string
	}

	gofishClient struct {
		config *BMCDetail
		client *gofish.APIClient
	}
)

// NewClient creates a new Redfish client with the given BMC configuration
func NewClient(config *BMCDetail) *gofishClient {
	return &gofishClient{
		config: config,
	}
}

// Connect establishes a connection to the Redfish BMC
func (c *gofishClient) Connect(ctx context.Context) error {
	// Validate credentials - if one is provided, both must be provided
	if (c.config.Username == "" && c.config.Password != "") ||
		(c.config.Username != "" && c.config.Password == "") {
		return fmt.Errorf("both username and password must be provided for authentication")
	}

	// Create HTTP client with timeout and TLS configuration
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Configure TLS settings if insecure flag is set
	if c.config.Insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Configure Gofish client
	gofishConfig := gofish.ClientConfig{
		Endpoint:   c.config.Endpoint,
		Username:   c.config.Username,
		Password:   c.config.Password,
		HTTPClient: httpClient,
	}

	// Connect with context timeout
	client, err := gofish.ConnectContext(ctx, gofishConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to BMC at %s: %w", c.config.Endpoint, err)
	}

	c.client = client
	return nil
}

// Disconnect closes the connection to the Redfish BMC
func (c *gofishClient) Disconnect() {
	if c.client != nil {
		c.client.Logout()
		c.client = nil
	}
}

// IsConnected returns true if the client is connected
func (c *gofishClient) IsConnected() bool {
	return c.client != nil
}

// GetAPIClient returns the underlying Gofish API client
// This should only be called after a successful Connect()
func (c *gofishClient) GetAPIClient() *gofish.APIClient {
	return c.client
}

// Endpoint returns the BMC endpoint URL (for logging purposes, never includes credentials)
func (c *gofishClient) Endpoint() string {
	return c.config.Endpoint
}
