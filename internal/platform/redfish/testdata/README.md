# Redfish Test Data

This package contains test fixtures and validation utilities for Redfish BMC testing.

## Test Data Sources and Validation

### How We Ensure Test Data Correctness

1. **Schema Validation**: All fixtures are validated against official gofish structs
2. **Real BMC Capture**: Fixtures are derived from real BMC responses
3. **Automated Validation**: CI runs validation tests on all fixtures
4. **Vendor Testing**: Fixtures cover major BMC vendors (Dell, HPE, Lenovo)

### Fixture Categories

#### Power Response Fixtures

- `dell_power_245w` - Dell iDRAC power response (245W consumption)
- `hpe_power_189w` - HPE iLO power response (189.5W consumption)
- `lenovo_power_167w` - Lenovo XCC power response (167.8W consumption)
- `generic_power_200w` - Generic Redfish-compliant response (200W)
- `zero_power` - Zero power consumption scenario
- `empty_power_control` - Missing PowerControl array

#### Infrastructure Fixtures

- `service_root` - Redfish service root response
- `chassis_collection` - Chassis collection response
- `chassis` - Individual chassis response

#### Error Fixtures

- `error_not_found` - HTTP 404 resource not found
- `error_auth_failed` - Authentication failure

### Validation Process

```bash
# Run fixture validation tests
go test ./internal/platform/redfish/testdata -v

# Validate individual fixture
go test -run TestIndividualFixtures/DellPower
```

### Capturing Real BMC Data

For development purposes, you can capture real BMC responses:

```bash
# Build with manual tag to include real capture utilities
go test -tags=manual ./internal/platform/redfish/testdata -run TestCapture
```

**Security Note**: Real capture utilities sanitize sensitive data (UUIDs, serial numbers, IPs) before creating fixtures.

### Adding New Fixtures

1. **From Real BMC**: Use `real_capture.go` utilities to capture authentic responses
2. **Manual Creation**: Create JSON following Redfish schema patterns
3. **Validation**: Ensure new fixtures pass `ValidateFixture()` tests
4. **Testing**: Add test scenarios using the new fixture

### Fixture Structure Guidelines

#### Power Fixtures Must Include

- `@odata.type`: Redfish Power schema type
- `Id`: Resource identifier
- `PowerControl`: Array with at least one power control object
- `PowerConsumedWatts`: Current power consumption value

#### Error Fixtures Must Include

- `error`: Object with error details
- `code`: Redfish error code
- `message`: Human-readable error message

### API Evolution Strategy

When Redfish API changes:

1. **Schema Updates**: Update gofish dependency to latest version
2. **Fixture Migration**: Use validation tests to identify incompatible fixtures
3. **Real Data Refresh**: Re-capture from updated BMCs when possible
4. **Backward Compatibility**: Maintain old fixtures for testing legacy scenarios

### Best Practices

- **Minimal Fixtures**: Include only necessary fields for test scenarios
- **Vendor Diversity**: Test against multiple BMC vendor formats
- **Error Coverage**: Include various error conditions
- **Real-World Data**: Base fixtures on actual BMC responses when possible
- **Security**: Never include real credentials, serial numbers, or network details

### Running Validation

Validation is automatically run in CI, but you can run it locally:

```bash
# Validate all fixtures
go test ./internal/platform/redfish/testdata -run TestFixtureValidation

# Validate specific vendor fixtures
go test ./internal/platform/redfish/testdata -run TestIndividualFixtures/Dell

# Check for JSON syntax errors
go test ./internal/platform/redfish/testdata -run TestErrorFixtures
```

### Integration with Gofish

Our fixtures leverage gofish's approach:

- **Struct Compatibility**: All fixtures validate against gofish structs
- **Error Handling**: Use gofish's error response patterns
- **Schema Compliance**: Follow DMTF Redfish schema standards
- **Vendor Support**: Cover vendor-specific response variations

This ensures our tests accurately represent real-world BMC behavior while maintaining reliability and maintainability.
