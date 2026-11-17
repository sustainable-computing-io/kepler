# EP-XXX: Explicit Reading Kind for Energy Zones

**Status**: Draft
**Authors**: @cvas22
**Reviewers**: @bitflicker64, @vimalk78
**Approvers**: @sthaha, @vprashar2929
**Created**: 2026-11-16
**Last Updated**: 2026-11-16

## Summary

`EnergyZone` was designed for RAPL, which reports a cumulative energy counter. When
hwmon support added sensors that report instantaneous power (#2365), the interface
gained a `Power()` method, and the review agreed that zones should instead say which
kind of value they provide, in a separate change after an Enhancement Proposal. This
is that proposal. It adds a `Kind()` method to `EnergyZone`, so the power monitor
reads each zone through the one method that matches its kind instead of reading both
and guessing. Tracked in #2640.

## Problem Statement

Every zone implements both `Energy()` and `Power()` and returns an error from the one
it doesn't support. `internal/monitor/node.go` calls both on every zone, every cycle,
in `firstNodeRead` and in the regular update, and then decides what the zone is:

```go
energyReading, energyErr := zone.Energy()
powerReading, powerErr := zone.Power()

// hwmon zones have MaxEnergy() == 0 and Energy() returns 0
isEnergySensor := zone.MaxEnergy() > 0 || energyReading > 0
```

The guess holds for RAPL, whose zones report a non-zero `MaxEnergy()`, and for hwmon
`power*_input` and voltage/current zones, whose `Energy()` always fails. It doesn't
hold for hwmon `energy*_input` zones (`hwmonEnergyZone`), which are cumulative but
report `MaxEnergy() == 0` because hwmon doesn't expose a wrap point. For those, the
kind depends on the value just read.

### Current Limitations

1. **An energy sensor that reads 0 is treated as a power sensor for that cycle.** Its
   `Power()` returns an error, so the zone is dropped from the cycle with the warning
   "Could not read power for zone", which is not what happened.
2. **A failed energy read is reported as a failed power read.** When `Energy()` fails
   on an hwmon energy zone, the reading is 0, the zone is classified as a power
   sensor, and the warning names the power read. The real error is lost.
3. **Every zone is read twice per cycle.** One read always fails, which builds an
   error that is thrown away. The unsupported method returns a constant error and
   doesn't touch sysfs, so the cost is the discarded error, not an extra read.
4. **New meters have no clear contract.** The IIO power meter for Jetson (#2438) and
   the MSR fallback (EP-002) each have to implement both methods and rely on the
   heuristic classifying them correctly.

## Goals

- **Primary Goal**: Each zone declares whether it reports cumulative energy or
  instantaneous power, and the monitor uses that instead of inferring it.
- **Secondary Goal**: Read each zone once per cycle, through the matching method.
- **Secondary Goal**: Report read errors against the method that actually failed.
- **Secondary Goal**: Give new power meters one explicit contract to implement.

## Non-Goals

- Replacing `EnergyZone` with a new untyped sensor interface. See "Alternatives
  Considered".
- Changing how power is attributed to processes, containers, pods or VMs.
- Changing the GPU or Redfish paths, which don't use `EnergyZone`.
- Discovering a wrap point for hwmon energy counters.
- Changing exported metrics or configuration, except for the zone names on hosts
  that have same-named zones of different kinds (see Operational Risks).

## Requirements

### Functional Requirements

- `EnergyZone` exposes the kind of value the zone reports: cumulative energy or
  instantaneous power.
- Every implementation in the tree returns a fixed kind: RAPL sysfs zones, hwmon
  power, voltage/current and energy zones, the fake meter's zones, `AggregatedZone`,
  and the test mocks.
- The monitor calls `Energy()` only on energy zones and `Power()` only on power zones.
- An energy zone whose counter reads 0 is still processed as an energy zone.
- `NewAggregatedZone` rejects members of different kinds.
- hwmon groups zones for aggregation by name and kind, so an `energy*_input` and a
  `power*_input` zone that share a label are never aggregated together.

### Non-Functional Requirements

- **Performance**: No call that is known to fail is made per zone per cycle.
- **Reliability**: A zone's kind can't change between cycles.
- **Maintainability**: The kind is visible in the type that produces the reading,
  not reconstructed from the reading.
- **Testability**: The monitor's handling of each kind can be tested with a mock
  zone that sets the kind directly.

## Proposed Solution

### High-Level Architecture

```text
power meter (rapl / hwmon / fake / future IIO, MSR)
        |
        v
  EnergyZone  ---- Kind() -----------------------------+
        |                                              |
        v                                              v
  monitor/node.go                                   switch kind
        |                       ZoneKindEnergy: Energy(), delta against the
        |                                       previous reading, wrap at MaxEnergy()
        |                       ZoneKindPower:  Power(), integrate over the interval
        v
  NodeUsage (unchanged)
```

### Key Design Choices

- **Add a method, keep the typed readings.** `Energy()` and `Power()` already return
  distinct types (`Energy`, `Power`) with unit helpers. Keeping them, and adding
  `Kind()` next to them, fixes the classification without touching the units or the
  attribution code that consumes them. That's the part the #2365 discussion flagged
  as making a full rewrite large.
- **The kind is fixed per zone.** It's decided when the meter builds the zone, from
  what the hardware exposes (`energy_uj`, `energy*_input`, `power*_input`,
  `in*_input` plus `curr*_input`), and never from a reading.
- **The unsupported method stays.** Zones keep returning an error from the method
  that doesn't match their kind. The monitor no longer calls it, but keeping it
  avoids splitting the interface in this step and keeps out-of-tree implementations
  compiling once they add `Kind()`.

## Detailed Design

### Package Structure

```text
internal/
├── device/
│   ├── cpu_power_meter.go         # ZoneKind, EnergyZone.Kind()
│   ├── energy_zone.go             # AggregatedZone.Kind(), mixed-kind check
│   ├── rapl_sysfs_power_meter.go  # sysfsRaplZone.Kind() = energy
│   ├── hwmon_power_meter.go       # Kind() on the three hwmon zones, grouping by (name, kind)
│   ├── fake_cpu_power_meter.go    # fakeEnergyZone.Kind() = energy
│   └── mock_cpu_power_meter.go    # MockRaplZone = energy, MockPowerZone = power
└── monitor/
    ├── node.go                    # dispatch on Kind() in firstNodeRead and the update
    └── mock_utils.go              # MockEnergyZone gains a kind
```

### API/Interface Changes

```go
// ZoneKind says which kind of value a zone reports.
type ZoneKind int

const (
    // ZoneKindEnergy is a cumulative energy counter in microjoules, read with
    // Energy(). It may wrap at MaxEnergy(); 0 means the wrap point is unknown.
    ZoneKindEnergy ZoneKind = iota
    // ZoneKindPower is an instantaneous power reading in microwatts, read with Power().
    ZoneKindPower
)

func (k ZoneKind) String() string // "energy", "power"

type EnergyZone interface {
    Name() string
    Index() int
    Path() string

    // Kind returns which kind of value the zone reports. It never changes.
    Kind() ZoneKind

    // Energy returns the cumulative energy. Only valid for ZoneKindEnergy.
    Energy() (Energy, error)

    // MaxEnergy returns the wrap point of Energy(), or 0 when it's unknown.
    // Only meaningful for ZoneKindEnergy.
    MaxEnergy() Energy

    // Power returns the instantaneous power. Only valid for ZoneKindPower.
    Power() (Power, error)
}
```

`AggregatedZone` takes its kind from its members. `NewAggregatedZone` returns an
error when they differ. Its `Power()` keeps summing member readings, and its
`Energy()` keeps its wrap handling, but each is only called for the matching kind.

In `internal/monitor/node.go`, both `firstNodeRead` and the update replace the
double read and the heuristic with:

```go
switch zone.Kind() {
case device.ZoneKindEnergy:
    energyReading, err := zone.Energy()
    // existing energy path; err is reported as an energy read failure
case device.ZoneKindPower:
    powerReading, err := zone.Power()
    // existing power path; err is reported as a power read failure
}
```

The delta, integration and active/idle split that follow are unchanged. The debug
log's `type` field uses `zone.Kind().String()`.

## Configuration

No new options. The kind comes from the hardware interface the meter already
selects.

`experimental.hwmon.zones` keeps matching the raw hwmon label, since `filterZones`
runs before `groupZonesByName`. So `core` selects both the energy zone and the
suffixed `core_power` zone built from the same label, and a filter copied from the
exported `zone` label, such as `core_power`, matches nothing. The config docs and
the release note will say the filter takes the hwmon label, not the exported name.

### Main Configuration Changes

None.

### New Configuration File (if applicable)

None.

### Security Considerations

None. No new files are read.

## Deployment Examples

No deployment changes. The change is internal to the CPU power meters and the
monitor.

### Kubernetes Environment

Unchanged.

### Standalone Deployment

Unchanged.

## Testing Strategy

### Test Coverage

- **Unit Tests**:
  - Each zone type returns the expected kind.
  - `NewAggregatedZone` rejects mixed kinds and reports the kind of uniform members.
  - hwmon grouping keeps an energy and a power zone with the same label apart.
  - Monitor, energy zone reading 0: processed as energy, no power read, no warning.
    This fails today.
  - Monitor, energy zone read error: reported as an energy read failure. Today it is
    reported as a power read failure.
  - Monitor: `Power()` is never called on an energy zone, and `Energy()` is never
    called on a power zone (mocks that fail the test if called).
  - Existing RAPL and hwmon power tests pass unchanged.
- **Integration Tests**: The existing monitor collection tests with the fake meter.
- **End-to-End Tests**: No change. Node and workload metrics stay the same on RAPL
  and hwmon power hosts.

### Test Infrastructure

`MockEnergyZone` in `internal/monitor/mock_utils.go` gets a configurable kind, so
monitor tests set it directly instead of shaping `MaxEnergy()` and readings to steer
the heuristic.

## Migration and Compatibility

### Backward Compatibility

No user-visible change for zones the heuristic already classified correctly, which
is every RAPL zone and every hwmon power or voltage/current zone. hwmon energy zones
stop dropping out of a cycle when they read 0. A zone whose `Energy()` read fails
still skips that cycle as it does today, but the failure is now reported as an energy
read failure instead of a power one.

`EnergyZone` is in `internal/`, so the new method only affects code in this
repository.

### Migration Path

1. **Phase 1**: Add `ZoneKind` and `Kind()`, implement it on every zone and mock. The
   monitor still uses the heuristic, and a test checks that `Kind()` agrees with it
   for every zone type the heuristic handles correctly.
2. **Phase 2**: Switch the monitor to dispatch on `Kind()`, add the mixed-kind check
   and the hwmon grouping by kind, and remove the heuristic.

### Rollback Strategy

Each phase is a single PR and can be reverted on its own. Reverting phase 2 brings
back the heuristic, which phase 1 leaves in place.

## Metrics Output

No new or changed metrics.

## Implementation Plan

### Phase 1: Foundation

- Add `ZoneKind` and `EnergyZone.Kind()`.
- Implement it for RAPL sysfs, hwmon (power, voltage/current, energy), fake, and
  `AggregatedZone`, plus the mocks.
- Tests for each zone's kind.

### Phase 2: Core Functionality

- Dispatch on `Kind()` in `firstNodeRead` and the update, and remove the heuristic.
- `NewAggregatedZone` rejects mixed kinds. hwmon groups by name and kind.
- Monitor tests for the zero reading, the error attribution and the single read.

### Phase 3: Testing and Documentation

- Update `docs/developer/` where it describes how zones are read, and mention
  `Kind()` in the guidance for new power meters, for #2438 and EP-002.

## Risks and Mitigations

### Technical Risks

- **Risk**: A zone declares the wrong kind and is read through a method that always
  fails, so it never reports.
  - **Mitigation**: The kind is set where the zone is built from a specific sysfs
    file, and phase 1 checks that `Kind()` matches today's classification before the
    monitor relies on it.
- **Risk**: An hwmon energy counter with no known wrap point goes backwards (wrap or
  driver reset), and `calculateEnergyDelta` returns 0 for that cycle.
  - **Mitigation**: That's today's behavior for those zones whenever they're
    classified as energy, and it's out of scope here. Making `Kind()` explicit makes
    the case easy to find and handle later.

### Operational Risks

- **Risk**: hwmon hosts that today aggregate same-named zones of different kinds
  would now see two zones instead of one. Process, container, VM and pod CPU metrics
  carry only the `zone` label, so two zones with the same `Name()` would emit
  identical series and the registry would fail the whole `/metrics` scrape with
  "collected before with the same name and label values".
  - **Mitigation**: Zones split by kind get distinct names. When a label is shared
    by an energy and a power sensor, the zone built from the power sensor gets a
    kind suffix, for example `core_power`, and the energy zone keeps the bare label.
    hwmon labels can already end in `_power`, so the suffixed name also has to be
    unique among all zone names. If it clashes with another zone, the chip name from
    the hwmon `name` file is added, for example `core_power_<chip>`, and the hwmon
    index after that if two chips share a name. A test builds both kinds from one
    label and checks that the names differ, and another adds a power sensor already
    labelled `core_power` and checks that every zone name is unique. The
    old aggregation produces a zone whose `Energy()` and `Power()` both fail for at
    least one member, so it can't report correctly today. The release note calls
    out the new zone names.

## Alternatives Considered

### Alternative 1: Replace `EnergyZone` with a `Sensor` interface

- **Description**: The interface sketched in the #2365 review: `Type() SensorType`,
  a single `Read() (uint64, error)` in base units and `MaxValue() uint64`.
- **Reason for Rejection**: It solves the same problem, but it drops the `Energy`
  and `Power` types and their unit helpers in favor of an untyped `uint64`, and
  touches every meter, the aggregation and the attribution code at once. `Kind()`
  gets the explicit classification now. A later EP can still split or rename the
  interface on top of it.

### Alternative 2: Split into `EnergyZone` and `PowerZone` interfaces

- **Description**: Two interfaces with only the methods each kind supports, and
  meters return a list of either.
- **Reason for Rejection**: It removes the always-failing method, but `Zones()`,
  `PrimaryEnergyZone()`, the monitor's zone maps and `AggregatedZone` would all need
  a union type or a type switch, which is a larger change than this problem needs. It
  stays possible after `Kind()` exists.

### Alternative 3: Fix the heuristic

- **Description**: Treat a zone as energy when `Power()` fails, or cache the first
  classification per zone.
- **Reason for Rejection**: It still reads both methods every cycle, still depends on
  read results, and still gives new meters no contract to implement.

## Success Metrics

- **Functional Metric**: hwmon energy zones are processed on every cycle, including
  when they read 0, and read errors name the method that failed.
- **Performance Metric**: No zone makes a call that is known to fail each cycle.
- **Adoption Metric**: New meters (#2438, EP-002) implement `Kind()` instead of
  relying on the heuristic.

## Open Questions

1. Should `MaxEnergy()` move behind the energy kind, for example into a small
   `EnergyCounter` interface, or stay on `EnergyZone` with "only meaningful for
   energy zones" in its doc comment?
2. Is it worth a third kind for zones that can provide both, such as a future meter
   that reads an energy counter and also exposes a power average? The proposal
   assumes not: such a meter picks the kind it trusts more.
3. Is one EP for both phases right, or would maintainers rather review phase 1 as a
   plain PR without the EP?
