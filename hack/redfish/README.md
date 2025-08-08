# Redfish BMC Test Data Capture Tools

This directory contains utilities for capturing real BMC test data to improve Kepler's Redfish power monitoring capabilities.

## 🎯 Quick Start

```bash
# Capture from single BMC
go run hack/redfish/capture-bmc-testdata.go \
    -endpoint https://192.168.1.100 \
    -username admin \
    -password yourpassword \
    -vendor dell

# Capture using config file
go run hack/redfish/capture-bmc-testdata.go \
    -config hack/redfish/bmc-config.yaml \
    -node worker-node-1
```

## 📁 Files Overview

### `capture-bmc-testdata.go`

**🌟 Main capture utility** - Use this for all test data capture needs.

**Features:**

- ✅ Config file support for multiple BMCs
- ✅ Automatic data sanitization
- ✅ Mock server integration output
- ✅ Copy-paste ready code snippets
- ✅ Comprehensive error handling
- ✅ Security-conscious design

**Usage:**

```bash
# Command line flags
go run hack/redfish/capture-bmc-testdata.go [options]

# Environment variables
export BMC_ENDPOINT="https://192.168.1.100"
export BMC_USERNAME="admin"
export BMC_PASSWORD="password"
go run hack/redfish/capture-bmc-testdata.go

# Config file (recommended)
go run hack/redfish/capture-bmc-testdata.go -config bmc-config.yaml -node worker-1
```

### `bmc-config.yaml`

**Configuration template** for managing multiple BMCs.

**Format:**

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
```

### ~~`real_capture.go`~~ (Removed)

**Removed** - The deprecated legacy capture utility has been removed.

All functionality is now consolidated in `capture-bmc-testdata.go`.

## 🚀 Integration Workflow

1. **Capture BMC data**:

   ```bash
   go run hack/redfish/capture-bmc-testdata.go -endpoint ... -vendor dell
   ```

2. **Review sanitized output**:
   - Check that sensitive data is removed
   - Verify power readings are reasonable
   - Ensure vendor-specific formats are captured

3. **Integrate with mock server**:
   - Copy fixture to `internal/platform/redfish/mock/power_responses.go`
   - Add scenario to `internal/platform/redfish/mock/scenarios.go`
   - Add vendor constants if needed

4. **Test integration**:

   ```bash
   go test ./internal/platform/redfish/mock -v
   go test ./internal/platform/redfish -run TestPowerReader
   ```

## 🛠️ Supported Hardware

### Tested BMC Vendors

- **Dell iDRAC** (iDRAC9, iDRAC8)
- **HPE iLO** (iLO5, iLO6)
- **Lenovo XCC** (XClarity Controller)
- **Generic Redfish** (Standard implementations)

### Power Monitoring Features

- ✅ System-level power consumption
- ✅ Real-time power readings
- ✅ Chassis power information
- ✅ Power control data structures

## 🔒 Security Features

### Automatic Sanitization

The capture script automatically removes/replaces:

- **IP Addresses** → `192.0.2.1` (RFC5737 test range)
- **Serial Numbers** → `TEST-SERIAL-123456`
- **UUIDs** → `12345678-1234-1234-1234-123456789012`
- **MAC Addresses** → `00:11:22:33:44:55`
- **Asset Tags** → `TEST-ASSET-TAG`
- **Service Tags** → `TEST-SERVICE-TAG`

### Manual Review Checklist

Before contributing captured data:

- [ ] No real IP addresses remain
- [ ] No actual serial numbers or UUIDs
- [ ] No company-specific model numbers
- [ ] Power readings are anonymized
- [ ] No internal network information

## 📊 Output Examples

### Power Response Fixture

```go
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

### Test Scenario

```go
{
    Name: "Dell275W",
    Config: ServerConfig{
        Vendor:     VendorDell,
        Username:   "admin",
        Password:   "password",
        PowerWatts: 275.0,
        EnableAuth: true,
    },
    PowerWatts: 275.0,
},
```

## 🧪 Testing & Validation

After integration, run comprehensive tests:

```bash
# Mock server tests
go test ./internal/platform/redfish/mock -v

# Power reader integration
go test ./internal/platform/redfish -run TestPowerReader

# Vendor-specific tests
go test ./internal/platform/redfish -run TestServiceIntegrationWithDifferentVendors

# Full test suite with race detection
go test ./internal/platform/redfish/... -race
```

## 🐛 Troubleshooting

### Common Issues

**Connection timeouts:**

```bash
# Increase timeout
-timeout 60s

# Check connectivity
ping bmc-ip
curl -k https://bmc-ip/redfish/v1/
```

**Authentication failures:**

```bash
# Verify credentials
# Check account lockout
# Confirm power monitoring permissions
```

**TLS certificate errors:**

```bash
# Use insecure flag (normal for BMCs)
-insecure
```

### Debug Output

For troubleshooting, increase verbosity:

```bash
# Enable detailed logging
export DEBUG=1
go run hack/redfish/capture-bmc-testdata.go -endpoint ... -vendor ...
```

## 🤝 Contributing

### Test Data Contributions Welcome

We need test data for:

- New BMC vendors (Supermicro, ASUS, etc.)
- Different power ranges (idle, normal, peak)
- Various server models
- Error scenarios

### Contribution Process

1. Capture data using the script
2. Review security sanitization
3. Test integration locally
4. Create pull request with hardware details

### Pull Request Template

```markdown
feat(redfish): add Dell PowerEdge R750 BMC test data

- Server: Dell PowerEdge R750
- BMC: iDRAC9 firmware 6.10.30.00
- Power: 275.0 watts
- Security: All sensitive data sanitized
```

## 📚 Documentation

- **Full Guide**: [internal/platform/redfish/testdata/UPDATING_TEST_DATA.md](../../internal/platform/redfish/testdata/UPDATING_TEST_DATA.md)
- **Mock Server**: [internal/platform/redfish/mock/](../../internal/platform/redfish/mock/)
- **Kepler Configuration**: [hack/config.yaml](../config.yaml)

## ⚡ Advanced Usage

### Batch Processing

```bash
# Capture multiple BMCs
for node in node1 node2 node3; do
    go run hack/redfish/capture-bmc-testdata.go \
        -config bmc-config.yaml -node $node > "capture-$node.txt"
done
```

### Custom Sanitization

Modify `sanitizeJSON()` function for additional sanitization rules.

### Integration Testing

```bash
# Build and test with new fixtures
make build
sudo ./bin/kepler --config hack/config.yaml --dev.fake-cpu-meter.enabled
```

---

**🎉 Thank you for contributing to Kepler's BMC compatibility!**

Your test data helps ensure reliable power monitoring across diverse hardware environments.
