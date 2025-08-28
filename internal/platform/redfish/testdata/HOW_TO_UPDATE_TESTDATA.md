# Updating Redfish Test Data

This guide explains how to capture and integrate real BMC test data for the
Kepler Redfish power monitoring feature.

## Table of Contents

- [Quick Start](#quick-start)
- [Using the Capture Script](#using-the-capture-script)
- [Integration Process](#integration-process)
- [Testing and Validation](#testing-and-validation)
- [Security Guidelines](#security-guidelines)
- [Contributing Test Data](#contributing-test-data)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Prerequisites

- Go 1.23 or later
- Access to a Redfish BMC (Dell iDRAC, HPE iLO, Lenovo XCC, etc.)
- BMC credentials with read access to power information

### 1. Capture BMC Data

The easiest way to capture test data is using the automated capture script:

```bash
# Method 1: Direct connection with command line arguments
go run hack/redfish/capture-bmc-testdata.go \
    -endpoint https://192.168.1.100 \
    -username admin \
    -password yourpassword \
    -vendor dell

# Method 2: Using environment variables
export BMC_ENDPOINT="https://192.168.1.100"
export BMC_USERNAME="admin"
export BMC_PASSWORD="yourpassword"
export BMC_VENDOR="dell"
go run hack/redfish/capture-bmc-testdata.go

# Method 3: Using config file (recommended for multiple BMCs)
go run hack/redfish/capture-bmc-testdata.go \
    -config hack/redfish/bmc-config.yaml \
    -node worker-node-1
```

### 2. Review Output

The script will:

1. Connect to your BMC safely
2. Capture power monitoring data
3. Automatically sanitize sensitive information
4. Generate mock server integration code
5. Provide copy-paste ready snippets

Example output:

```text
📊 Capture Results Summary
==========================
🏭 Vendor: dell
🖥️  BMC Model: iDRAC9 (FW: 6.10.30.00)
🏗️  Server Model: PowerEdge R750
⚡ Power Consumption: 275.0 watts

📝 Generated Mock Server Integration
====================================

// 1. Add this to internal/platform/redfish/mock/power_responses.go:
// In the PowerResponseFixtures map:
"dell_power_275w": `{...real BMC response data...}`,
```

## Using the Capture Script

### Configuration File Setup

For managing multiple BMCs, create a config file (see `hack/redfish/bmc-config.yaml`):

```yaml
nodes:
  worker-node-1: bmc-1
  worker-node-2: bmc-2
bmcs:
  bmc-1:
    endpoint: https://192.168.1.100
    username: admin
    password: secret123
    insecure: true
  bmc-2:
    endpoint: https://192.168.1.101
    username: admin
    password: secret456
    insecure: true
```

### Command Line Options

```text
Usage: capture-bmc-testdata.go [options]

Options:
  -config string
        Path to BMC configuration YAML file
  -endpoint string
        BMC endpoint URL (or set BMC_ENDPOINT env var)
  -insecure
        Skip TLS verification (recommended for testing) (default true)
  -node string
        Node name to capture (when using config file)
  -password string
        BMC password (or set BMC_PASSWORD env var)
  -timeout duration
        Connection timeout (default 30s)
  -username string
        BMC username (or set BMC_USERNAME env var)
  -vendor string
        BMC vendor: dell, hpe, lenovo, or generic (default "generic")
```

### Supported Vendors

- **Dell**: iDRAC (tested with iDRAC9)
- **HPE**: iLO (tested with iLO5/6)
- **Lenovo**: XClarity Controller (XCC)
- **Generic**: Standard Redfish implementations

## Integration Process

The capture script generates ready-to-integrate code snippets:

### Step 1: Add Power Response Fixture

Copy the generated fixture to `internal/platform/redfish/mock/power_responses.go`:

```go
// Add to PowerResponseFixtures map:
"dell_power_275w": `{
    "@odata.context": "/redfish/v1/$metadata#Power.Power",
    "@odata.type": "#Power.v1_5_0.Power",
    "@odata.id": "/redfish/v1/Chassis/1/Power",
    "Id": "Power",
    "Name": "Power",
    "PowerControl": [
        {
            "@odata.id": "/redfish/v1/Chassis/1/Power#/PowerControl/0",
            "Name": "System Power Control",
            "PowerConsumedWatts": 275.0
        }
    ]
}`,
```

### Step 2: Add Test Scenario

Add the scenario to `internal/platform/redfish/mock/scenarios.go`:

```go
// Add to GetSuccessScenarios() return slice:
{
    Name: "Dell275W",
    Config: ServerConfig{
        Vendor:     VendorDell,
        Username:   baseConfig.Username,
        Password:   baseConfig.Password,
        PowerWatts: 275.0,
        EnableAuth: baseConfig.EnableAuth,
    },
    PowerWatts: 275.0,
},
```

### Step 3: Add Vendor Support (if new vendor)

If adding a new vendor, update `internal/platform/redfish/mock/server.go`:

```go
// Add vendor constant
VendorSupermicro VendorType = "supermicro"
```

And add to vendor lists in tests:

```go
vendors := []mock.VendorType{
    mock.VendorDell,
    mock.VendorHPE,
    mock.VendorLenovo,
    mock.VendorSupermicro, // New vendor
    mock.VendorGeneric,
}
```

## Testing and Validation

### Run Validation Tests

After integration, verify everything works:

```bash
# Test mock server functionality
go test ./internal/platform/redfish/mock -v

# Test power reader with new fixtures
go test ./internal/platform/redfish -run TestPowerReader

# Test vendor integration
go test ./internal/platform/redfish -run TestServiceIntegrationWithDifferentVendors

# Run all Redfish tests
go test ./internal/platform/redfish/... -race
```

### Verify Integration

The new fixtures should work with existing tests automatically:

```bash
# Check that vendor variations work
go test ./internal/platform/redfish -run TestPowerReaderReadPowerVendorVariations

# Verify service lifecycle
go test ./internal/platform/redfish -run TestServicePowerDataCollection
```

## Security Guidelines

### Automatic Sanitization

The capture script automatically sanitizes:

- ✅ **IP addresses** → `192.0.2.1` (test IP range)
- ✅ **Serial numbers** → `TEST-SERIAL-123456`
- ✅ **UUIDs** → `12345678-1234-1234-1234-123456789012`
- ✅ **MAC addresses** → `00:11:22:33:44:55`
- ✅ **Asset/Service tags** → `TEST-ASSET-TAG`

### Manual Review Required

Always review captured data before sharing:

1. **Check for missed sensitive data**: Company names, specific model numbers
2. **Verify power readings**: Ensure they're representative but not identifying
3. **Review error messages**: Remove any path-specific or internal information

### Security Checklist

Before contributing:

- [ ] No real IP addresses remain
- [ ] No actual serial numbers or UUIDs
- [ ] No credentials or authentication tokens
- [ ] No company-specific identifying information
- [ ] Power readings are realistic but anonymized

## Contributing Test Data

### What We Need

To improve Kepler's BMC compatibility, we welcome test data for:

- **New BMC vendors** (Supermicro, ASUS, etc.)
- **Different power consumption ranges** (idle, normal, peak)
- **Various server models** within existing vendors
- **Error scenarios** (authentication failures, missing capabilities)

### Contribution Process

1. **Capture data** using the script
2. **Review security** sanitization
3. **Test integration** locally
4. **Create pull request** with:
   - Title: `feat(redfish): add [Vendor] [Model] BMC test data`
   - Description including hardware details
   - Test coverage information

### Pull Request Template

```markdown
## Summary

Add real BMC test data for [Vendor] systems.

## Hardware Details

- **Server**: Dell PowerEdge R750
- **BMC**: iDRAC9 firmware 6.10.30.00
- **Power Reading**: 275.0 watts

## Test Coverage

- Power monitoring via Redfish
- Vendor-specific response format
- Authentication and connection handling

## Security

- All sensitive data sanitized
- No real IP addresses, serials, or UUIDs
- Reviewed for company-identifying information

## Testing

- [ ] Mock server tests pass
- [ ] Power reader tests work with new fixture
- [ ] Vendor integration tests include new data
```

## Troubleshooting

### Common Issues

#### 1. Connection Timeouts

```bash
Error: failed to connect to BMC: context deadline exceeded
```

**Solutions:**

- Check network connectivity: `ping bmc-ip`
- Verify BMC is accessible: `curl -k https://bmc-ip/redfish/v1/`
- Increase timeout: `-timeout 60s`
- Check firewall rules

#### 2. Authentication Failures

```bash
Error: 401 Unauthorized
```

**Solutions:**

- Verify credentials are correct
- Check if account is locked
- Try different authentication methods
- Confirm read permissions for power data

#### 3. TLS Certificate Issues

```bash
Error: x509: certificate signed by unknown authority
```

**Solution:**

- Use `-insecure` flag (recommended for testing)
- This is normal for BMCs with self-signed certificates

#### 4. Missing Power Data

```bash
Error: failed to get power data
```

**Solutions:**

- Verify BMC supports power monitoring
- Check hardware power management features are enabled
- Try different chassis (some servers have multiple)

#### 5. Gofish Compatibility Issues

```bash
Error: json: cannot unmarshal ... into Go struct field
```

**Solutions:**

- Update gofish: `go get -u github.com/stmcginnis/gofish`
- Check if BMC uses non-standard Redfish extensions
- Review captured JSON for schema compliance

### Getting Help

- **GitHub Issues**: Report problems or questions
- **GitHub Discussions**: General questions about test data
- **Security Issues**: Contact maintainers privately for security-related concerns

### Debug Mode

Enable verbose output for troubleshooting:

```bash
# Add debug logging (modify script to enable)
export DEBUG=1
go run hack/redfish/capture-bmc-testdata.go -endpoint ... -vendor ...
```

## Advanced Usage

### Batch Capture

Capture from multiple BMCs:

```bash
# Create script for batch processing
for node in worker-node-1 worker-node-2 worker-node-3; do
    echo "Capturing $node..."
    go run hack/redfish/capture-bmc-testdata.go \
        -config hack/redfish/bmc-config.yaml \
        -node $node > "capture-$node.txt"
done
```

### Custom Sanitization

For additional sanitization needs, modify the `sanitizeJSON` function:

```go
// Add custom sanitization rules
func sanitizeJSON(jsonStr string) string {
    // Existing sanitization...

    // Custom company-specific sanitization
    jsonStr = strings.ReplaceAll(jsonStr, "YourCompany", "TestCompany")

    return jsonStr
}
```

### Integration Testing

Test new fixtures against real Kepler deployment:

```bash
# Build Kepler with new test data
make build

# Test with mock BMC
sudo ./bin/kepler --config hack/config.yaml --dev.fake-cpu-meter.enabled
```

---

**Remember**: Quality test data ensures Kepler works reliably across diverse hardware environments. Your contributions help improve BMC compatibility for the entire community!
