# Updating Redfish Test Fixtures

This guide explains how to update test fixtures for the Kepler Redfish power monitoring feature.

## Table of Contents

- [Quick Start](#quick-start)
- [Integration Process](#integration-process)
- [Testing and Validation](#testing-and-validation)
- [Security Guidelines](#security-guidelines)

## Quick Start

### Prerequisites

- Go 1.23 or later
- For new test data: Access to a Redfish BMC and credentials

### 1. Capture BMC Data (for new fixtures)

Use the capture tool in `hack/redfish/` - see [hack/redfish/README.md](../../../../hack/redfish/README.md) for detailed instructions:

```bash
# Capture from BMC
go run hack/redfish/capture-bmc-testdata.go \
    -endpoint https://192.168.1.100 \
    -username admin \
    -password yourpassword \
    -vendor dell
```

### 2. Integration Process

The capture script generates ready-to-integrate JSON fixtures and code snippets for immediate use.

## Integration Process

### Step 1: Save JSON Fixture

Save the captured JSON data as a fixture file:

```bash
# Create fixture file in fixtures/ directory
echo '{...captured BMC response...}' > fixtures/dell_power_275w.json
```

### Step 2: Use Fixture in Tests

The fixture will be automatically loaded by the testdata package:

```go
// Reference fixture by filename (without .json extension)
response := CreateSuccessResponse("dell_power_275w")
powerReader := NewTestPowerReader(t, map[string]*http.Response{
    "/redfish/v1/Chassis/1/Power": response,
})

reading, err := powerReader.ReadPower(context.Background())
AssertPowerReading(t, 275.0, reading)
```

### Step 3: Add Test Scenario (Optional)

For comprehensive testing, add scenario to mock server in `internal/platform/redfish/mock/scenarios.go`:

```go
{
    Name: "Dell275W",
    Config: ServerConfig{
        Vendor:     VendorDell,
        PowerWatts: 275.0,
        EnableAuth: true,
    },
    PowerWatts: 275.0,
},
```

## Testing and Validation

### Run Validation Tests

After adding fixtures, verify they work:

```bash
# Test fixture loading
go test ./internal/platform/redfish/testdata -v

# Test power reader with new fixtures
go test ./internal/platform/redfish -run TestPowerReader -v

# Run all Redfish tests
go test ./internal/platform/redfish/... -race
```

## Security Guidelines

### Automatic Sanitization

The capture script (see [hack/redfish/README.md](../../../../hack/redfish/README.md)) automatically sanitizes sensitive data.

### Security Checklist

Before contributing fixtures:

- [ ] No real IP addresses, serial numbers, or UUIDs
- [ ] No credentials or authentication tokens
- [ ] No company-specific identifying information
- [ ] Power readings are realistic but anonymized

---

**For capturing new BMC test data, see [hack/redfish/README.md](../../../../hack/redfish/README.md)**
