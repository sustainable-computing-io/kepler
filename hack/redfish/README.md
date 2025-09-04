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
- ✅ Test-ready JSON fixtures
- ✅ Copy-paste ready code snippets
- ✅ Comprehensive error handling
- ✅ Security-conscious design

**Usage:**

```bash
# Command line flags
go run hack/redfish/capture-bmc-testdata.go [options]

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

## 🚀 Integration Workflow

1. **Capture BMC data**:

   ```bash
   go run hack/redfish/capture-bmc-testdata.go -endpoint ... -vendor dell
   ```

2. **Review sanitized output**:
   - Check that sensitive data is removed
   - Verify power readings are reasonable
   - Ensure vendor-specific formats are captured

3. **Integrate with test fixtures**:
   - Save fixture as JSON file in `internal/platform/redfish/testdata/fixtures/`
   - Use the fixture name in your tests (automatically loaded by testdata package)
   - Add vendor constants if needed

4. **Test integration**:

   ```bash
   # Test fixture loading and validation
   go test ./internal/platform/redfish/testdata -v

   # Test power reader functionality
   go test ./internal/platform/redfish -run TestPowerReader -v
   ```

## 🛠️ Supported Hardware

### To Capture BMC Vendors

- [x] **Generic Redfish** (Standard implementations using [sushy tool](https://docs.openstack.org/sushy-tools/latest/index.html))
- [ ] **Dell iDRAC** (iDRAC9, iDRAC8)
- [ ] **HPE iLO** (iLO5, iLO6)
- [ ] **Lenovo XCC** (XClarity Controller)

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

### Power Response Fixture (JSON File)

Save as `internal/platform/redfish/testdata/fixtures/dell_power_275w.json`:

```json
{
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
}
```

### Using the Fixture in Tests

Reference the fixture by name in your tests:

```go
// The fixture will be automatically loaded from dell_power_275w.json
response := CreateSuccessResponse("dell_power_275w")
powerReader := NewTestPowerReader(t, map[string]*http.Response{
    "/redfish/v1/Chassis/1/Power": response,
})

reading, err := powerReader.ReadPower(context.Background())
AssertPowerReading(t, 275.0, reading)
```

## 🧪 Testing & Validation

After integration, run comprehensive tests:

```bash
# Test fixture loading and validation
go test ./internal/platform/redfish/testdata -v

# Test power reader functionality
go test ./internal/platform/redfish -run TestPowerReader -v

# Test service integration
go test ./internal/platform/redfish -run TestServiceIntegrationWithDifferentVendors -v

# Full test suite with race detection
go test ./internal/platform/redfish/... -race
```

## 🐛 Troubleshooting

### Common Issues

> Add common issues here

## 🤝 Contributing

### Test Data Contributions Welcome

We need test data for:

- BMC vendors
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

- **Full Guide**: [internal/platform/redfish/testdata/HOW_TO_UPDATE_TESTDATA.md](../../internal/platform/redfish/testdata/HOW_TO_UPDATE_TESTDATA.md)
- **Test Fixtures**: [internal/platform/redfish/testdata/fixtures/](../../internal/platform/redfish/testdata/fixtures/)
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
