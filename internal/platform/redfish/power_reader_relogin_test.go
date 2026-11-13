// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package redfish

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stmcginnis/gofish/schemas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	redfishcfg "github.com/sustainable-computing-io/kepler/config/redfish"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish/testutil"
)

func newSessionServer(t *testing.T, watts float64) (*testutil.Server, *PowerReader) {
	t.Helper()

	server := testutil.NewServer(testutil.ServerConfig{
		Username:            "admin",
		Password:            "password",
		PowerWatts:          watts,
		EnableAuth:          true,
		RequireSessionToken: true,
	})
	t.Cleanup(server.Close)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	bmc := &redfishcfg.BMCDetail{
		Endpoint: server.URL(),
		Username: "admin",
		Password: "password",
		Insecure: true,
	}
	reader := NewPowerReader(bmc, 5*time.Second, logger)
	require.NoError(t, reader.Init())
	t.Cleanup(reader.Close)

	return server, reader
}

func totalWatts(chassis []Chassis) float64 {
	total := 0.0
	for _, c := range chassis {
		for _, r := range c.Readings {
			total += r.Power.Watts()
		}
	}
	return total
}

func TestPowerReaderReadAllLogsInAgainAfterSessionExpires(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	chassis, err := reader.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, chassis)
	assert.Equal(t, 1, server.LoginCount())

	server.ExpireSessions()
	server.SetPowerWatts(250)

	chassis, err = reader.ReadAll()
	require.NoError(t, err, "ReadAll should log in again when the BMC rejects the session")
	require.NotEmpty(t, chassis)
	assert.Equal(t, 2, server.LoginCount())

	// The new session keeps working without another login
	chassis2, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, totalWatts(chassis), totalWatts(chassis2))
	assert.Equal(t, 2, server.LoginCount())
}

func TestPowerReaderReadAllBacksOffAfterFailedLogin(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	server.ExpireSessions()
	server.SetError(testutil.ErrorAuth) // login is rejected too

	_, err := reader.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log in to BMC again")
	assert.NotNil(t, reader.client, "old client should be kept so a later read retries")
	assert.Equal(t, 2, server.LoginAttempts())

	server.SetError(testutil.ErrorNone)

	// Still inside the backoff, so no new login
	_, err = reader.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "next login attempt in")
	assert.Equal(t, 2, server.LoginAttempts(), "no login should be tried during the backoff")

	// Once the backoff is over the next read logs in and recovers
	reader.lastReloginFailure = time.Now().Add(-defaultReloginBackoff)

	chassis, err := reader.ReadAll()
	require.NoError(t, err, "ReadAll should recover once the BMC accepts logins again")
	require.NotEmpty(t, chassis)
	assert.Equal(t, 3, server.LoginAttempts())
	assert.True(t, reader.lastReloginFailure.IsZero(), "a good login should clear the backoff")
}

func TestPowerReaderReadAllBacksOffWhenFreshSessionIsRejected(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	// The login works, but the new session is dropped right away
	server.ExpireSessions()
	server.ExpireSessionsAfter("/redfish/v1/SessionService/Sessions")

	_, err := reader.ReadAll()
	require.Error(t, err)
	assert.Equal(t, 2, server.LoginAttempts())
	assert.False(t, reader.lastReloginFailure.IsZero(), "a rejected fresh session should start the backoff")

	_, err = reader.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "next login attempt in")
	assert.Equal(t, 2, server.LoginAttempts(), "no login should be tried during the backoff")
}

func TestPowerReaderReadAllLogsInAgainWhenChassisReadIsRejected(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	// The chassis list works, then the session dies before the power read
	server.ExpireSessionsAfter("/redfish/v1/Chassis/1")

	chassis, err := reader.ReadAll()
	require.NoError(t, err, "a 401 on a chassis read should also trigger a re-login")
	require.NotEmpty(t, chassis)
	assert.Equal(t, 2, server.LoginCount())
}

func TestPowerReaderReloginLogsOutOldSessionFirst(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	server.ExpireSessions()

	_, err := reader.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, []string{"login", "logout", "login"}, server.SessionEvents(),
		"the old session should be logged out before the new login")
}

func TestPowerReaderReadAllDoesNotLogInOnOtherErrors(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	server.SetError(testutil.ErrorInternalServer)

	_, err := reader.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get chassis collection")
	assert.Equal(t, 1, server.LoginCount(), "a 500 must not open a new BMC session")

	server.SetError(testutil.ErrorNone)
	_, err = reader.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, 1, server.LoginCount())
}

func TestPowerReaderReadAllConcurrentReloginLogsInOnce(t *testing.T) {
	server, reader := newSessionServer(t, 200)

	server.ExpireSessions()

	const readers = 8
	var wg sync.WaitGroup
	errs := make([]error, readers)
	for i := range readers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = reader.ReadAll()
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "reader %d", i)
	}
	assert.Equal(t, 2, server.LoginCount(), "only one re-login should happen")
}

func TestPowerReaderReadAllAfterClose(t *testing.T) {
	_, reader := newSessionServer(t, 200)

	_, err := reader.ReadAll()
	require.NoError(t, err)

	reader.Close()

	assert.NotPanics(t, func() {
		_, err := reader.ReadAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
	})
}

func TestPowerReaderCloseDuringReadAll(t *testing.T) {
	server, reader := newSessionServer(t, 200)
	server.ExpireSessions()

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = reader.ReadAll()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader.Close()
	}()
	wg.Wait()

	_, err := reader.ReadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestIsUnauthorized(t *testing.T) {
	unauthorized := schemas.ConstructError(http.StatusUnauthorized, []byte("Unauthorized"))
	serverErr := schemas.ConstructError(http.StatusInternalServerError, []byte("oops"))

	collWith := func(errs ...error) error {
		c := schemas.NewCollectionError()
		for i, e := range errs {
			c.Failures[fmt.Sprintf("/redfish/v1/Chassis/%d", i)] = e
		}
		return c
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("connection refused"), false},
		{"401", unauthorized, true},
		{"wrapped 401", fmt.Errorf("get chassis: %w", unauthorized), true},
		{"500", serverErr, false},
		{"collection with 401", collWith(serverErr, unauthorized), true},
		{"collection without 401", collWith(serverErr), false},
		{"wrapped collection with 401", fmt.Errorf("x: %w", collWith(unauthorized)), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isUnauthorized(tc.err))
		})
	}
}
