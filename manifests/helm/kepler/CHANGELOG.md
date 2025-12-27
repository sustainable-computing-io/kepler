# Changelog

All notable changes to the Kepler Helm chart will be documented in this file.

## [Unreleased]

### Added
- Dedicated health check endpoints for Kubernetes probes
  - Liveness probe now uses `/probe/livez` instead of `/metrics`
  - Readiness probe now uses `/probe/readyz` instead of being disabled
  - Improved probe timing and failure thresholds for better reliability

### Changed
- **BREAKING**: Health probes now use dedicated endpoints instead of `/metrics`
  - Liveness probe: `/metrics` → `/probe/livez`
  - Readiness probe: enabled with `/probe/readyz`
- Reduced liveness probe period from 60s to 30s for faster failure detection
- Added readiness probe with 10s period for better traffic management

### Technical Details
- Liveness probe checks if the monitor service is alive and collection is working
- Readiness probe checks if the monitor has data available to serve
- Both probes support passive mode (interval=0) and active collection modes
- Probe responses include JSON with status, timestamp, and duration information