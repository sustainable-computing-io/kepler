# EP-003: Virtual Machine CPU Power Consumption Modeling for Kepler

**Status**: Draft
**Author**: Kaiyi Liu
**Created**: 2025-08-25
**Last Updated**: 2025-08-25

## Summary

This proposal outlines a comprehensive approach for training and deploying machine learning models to estimate CPU power consumption in virtual machine environments for Kepler energy monitoring. The system will train models using VM-accessible OS and memory counters as features, with baremetal Kepler QEMU process power measurements as targets, enabling accurate power estimation for package, core, DRAM, and uncore zones in virtualized environments where direct hardware power measurements are unavailable.

## Problem Statement

Virtual machines lack direct access to hardware power measurement interfaces (RAPL, IPMI, etc.) that are essential for energy monitoring in cloud and virtualized environments. Current Kepler deployments in VMs cannot provide accurate power consumption estimates because they cannot access the underlying hardware power consumption data. This creates a significant gap in energy monitoring capabilities for the growing virtualized infrastructure landscape.

### Current Limitations

1. **No Direct Hardware Access**: VMs cannot access Intel RAPL or other hardware power measurement interfaces
2. **Missing Energy Metrics**: Kepler in VMs cannot report `kepler_node_cpu_watts` for package, core, DRAM, and uncore zones
3. **Limited Power Visibility**: Cloud providers and VM operators lack visibility into their actual energy consumption
4. **Inaccurate Estimations**: Existing VM power estimation methods are often generic and inaccurate for specific workloads
5. **Deployment Complexity**: No standardized approach for VM-based energy modeling in Kepler ecosystem

## Goals

- **Primary Goal**: Develop zone-specific machine learning models (package, core, DRAM, uncore) for CPU power estimation in VMs
- **Secondary Goal**: Create a production-ready deployment system for VM power models in Go environments
- **Tertiary Goal**: Establish best practices for VM power modeling including CPU pinning and isolation requirements
- **Performance Goal**: Achieve <10% mean absolute percentage error compared to baremetal measurements
- **Deployment Goal**: Enable seamless integration with existing Kepler infrastructure

## Non-Goals

- Modeling GPU or other accelerator power consumption
- Creating models for non-x86 architectures in this initial version (future implementation will include this)
- Replacing baremetal RAPL measurements where available
- Modeling power consumption for nested virtualization scenarios
- Supporting real-time inference with <1ms latency requirements

## Requirements

### Functional Requirements

- **FR1**: Train separate ML models for each power zone (package, core, DRAM, uncore)
- **FR2**: Use only VM-accessible OS and memory counters as input features
- **FR3**: Support model deployment and inference in Go applications
- **FR4**: Provide continuous training pipeline for model updates
- **FR5**: Include CPU/core pinning and isolation analysis for accuracy improvement
- **FR6**: Generate zone-specific `kepler_node_cpu_watts` metrics in VM environments
- **FR7**: Support multiple VM workload types (CPU-intensive, memory-intensive, I/O-bound, mixed)

### Non-Functional Requirements

- **Performance**: Model inference <100ms latency, training completion <4 hours
- **Reliability**: 99% model availability, graceful degradation on missing features
- **Security**: No exposure of sensitive system information, secure model storage
- **Maintainability**: Modular design, comprehensive logging, automated testing
- **Testability**: Unit tests >80% coverage, integration tests for all zones and model training

## Proposed Solution

### High-Level Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                    VM Environment (Guest)                       │
│  ┌───────────────┐    ┌─────────────────┐    ┌─────────────────┐│
│  │ OS Counters   │    │ Memory Counters │    │ Perf Counters   ││
│  │ /proc/stat    │    │ /proc/meminfo   │    │ CPU cycles,     ││
│  │ /proc/loadavg │    │ /proc/vmstat    │    │ instructions,   ││
│  │ CPU utilization│    │ Page faults     │    │ cache misses    ││
│  └───────────────┘    └─────────────────┘    └─────────────────┘│
│           │                      │                      │       │
│           └──────────────────────┼──────────────────────┘       │
│                                  │                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Feature Collection Service                    ││
│  │         (Collects VM-accessible metrics)                  ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   │ Feature Vector
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Baremetal Environment (Host)                   │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                   Kepler Process Monitor                   ││
│  │   Tracks QEMU process: kepler_process_cpu_watts           ││
│  │   - Zone: package, core, DRAM, uncore                    ││
│  │   - Target labels for ML training                        ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   │ Training Data
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Training Pipeline                           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │  Data Fusion    │  │  Feature Eng.   │  │  Model Training │ │
│  │  VM features +  │→ │  Normalization, │→ │  Zone-specific  │ │
│  │  Power labels   │  │  Derived metrics│  │  Models (4x)    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                   │
                                   │ Trained Models
                                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Deployment Environment                        │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              Go Inference Service                          ││
│  │  - Loads trained models                                   ││
│  │  - Collects VM features                                   ││
│  │  - Generates kepler_node_cpu_watts for each zone         ││
│  │  - Integrates with Kepler metrics pipeline               ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Key Design Choices

1. **Zone-Specific Models**: Separate models for package, core, DRAM, and uncore zones to capture distinct power characteristics
2. **VM-Only Features**: Exclusively use metrics available within VM environments to ensure deployment portability
3. **Process-Level Target**: Use baremetal Kepler's `kepler_process_cpu_watts` for QEMU processes as ground truth
4. **Go Integration**: Native Go inference to integrate seamlessly with Kepler's existing codebase
5. **Feature Engineering**: Emphasis on derived metrics and ratios rather than absolute values for better generalization
6. **Workload Diversity**: Training on multiple workload types to improve model robustness

## Detailed Design

### Package Structure

```text
vm_energy_modeling/
├── training/
│   ├── data_collection/
│   │   ├── vm_feature_collector.py      # VM metrics collection
│   │   ├── baremetal_monitor.py         # QEMU process monitoring
│   │   └── data_synchronizer.py         # Time-aligned data fusion
│   ├── feature_engineering/
│   │   ├── vm_feature_processor.py      # VM-specific feature processing
│   │   ├── derived_metrics.py           # Calculate efficiency ratios
│   │   └── normalization.py             # Feature scaling and normalization
│   ├── model_training/
│   │   ├── zone_trainers.py             # Zone-specific model training
│   │   ├── hyperparameter_optimization.py
│   │   └── model_validation.py          # Cross-validation and evaluation
│   └── workload_generation/
│       ├── cpu_intensive.py
│       ├── memory_intensive.py
│       ├── io_intensive.py
│       └── mixed_workloads.py
├── deployment/
│   ├── go_inference/
│   │   ├── model_loader.go              # Load Python models in Go
│   │   ├── feature_collector.go         # VM metrics collection in Go
│   │   ├── power_estimator.go           # Zone-specific power estimation
│   │   └── kepler_integration.go        # Metrics export to Kepler
│   ├── models/
│   │   ├── package_model.pkl
│   │   ├── core_model.pkl
│   │   ├── dram_model.pkl
│   │   └── uncore_model.pkl
│   └── python_bridge/
│       ├── model_server.py              # Python model inference server
│       └── grpc_interface.proto         # Go-Python communication
├── evaluation/
│   ├── accuracy_validation.py
│   ├── cross_platform_testing.py
│   └── performance_benchmarks.py
└── documentation/
    ├── cpu_pinning_guide.md
    ├── deployment_guide.md
    └── troubleshooting.md
```

### API/Interface Changes

**New Kepler Metrics in VM Environment:**

```go
// New VM-specific power metrics
type VMPowerMetrics struct {
    NodeCPUWatts map[string]float64 `json:"node_cpu_watts"` // zone -> watts
    ModelVersion string             `json:"model_version"`
    Confidence   float64            `json:"confidence"`
    Features     map[string]float64 `json:"features"`
}

// Zone definitions
const (
    ZonePackage = "package"
    ZoneCore    = "core"
    ZoneDRAM    = "dram"
    ZoneUncore  = "uncore"
)
```

**Go Inference Interface:**

```go
type PowerEstimator interface {
    EstimatePower(features VMFeatures) (VMPowerMetrics, error)
    LoadModels(modelPath string) error
    GetSupportedZones() []string
    GetModelVersion() string
}

type VMFeatures struct {
    CPUUtilization    float64 `json:"cpu_utilization"`
    CPUStealTime      float64 `json:"cpu_steal_time"`
    InstructionsPerCycle float64 `json:"instructions_per_cycle"`
    CacheMissRatio    float64 `json:"cache_miss_ratio"`
    MemoryUsagePercent float64 `json:"memory_usage_percent"`
    PageFaultRate     float64 `json:"page_fault_rate"`
    ContextSwitchRate float64 `json:"context_switch_rate"`
    // ... additional features
}
```

## Configuration

### Main Configuration Changes

```go
type VMEnergyConfig struct {
    Enabled           *bool   `yaml:"enabled"`
    ModelPath         string  `yaml:"modelPath"`
    CollectionInterval string  `yaml:"collectionInterval"`
    InferenceMethod   string  `yaml:"inferenceMethod"` // "native" or "python-bridge"
    PythonServerURL   string  `yaml:"pythonServerURL,omitempty"`
    CPUPinning        *CPUPinningConfig `yaml:"cpuPinning,omitempty"`
    FeatureConfig     *FeatureCollectionConfig `yaml:"featureConfig"`
}

type CPUPinningConfig struct {
    Enabled           *bool `yaml:"enabled"`
    IsolatedCPUs      []int `yaml:"isolatedCPUs"`
    RecommendationMode *bool `yaml:"recommendationMode"`
}

type FeatureCollectionConfig struct {
    PerfCountersEnabled *bool   `yaml:"perfCountersEnabled"`
    SamplingInterval    string  `yaml:"samplingInterval"`
    MetricsWhitelist    []string `yaml:"metricsWhitelist"`
}
```

### New Configuration File

```yaml
# /etc/kepler/vm-energy-config.yaml
vm_energy_modeling:
  enabled: true
  model_path: "/var/lib/kepler/vm-models/"
  collection_interval: "5s"
  inference_method: "native"  # or "python-bridge"

  cpu_pinning:
    enabled: false  # Set to true for high-accuracy requirements
    isolated_cpus: [2, 3, 6, 7]  # Example isolated CPU cores
    recommendation_mode: true  # Analyze and recommend optimal pinning

  feature_config:
    perf_counters_enabled: true
    sampling_interval: "1s"
    metrics_whitelist:
      - "cpu_utilization"
      - "instructions_per_cycle"
      - "cache_miss_ratio"
      - "memory_usage_percent"
      - "page_fault_rate"
      - "context_switch_rate"
      - "cpu_steal_time"

  # Python bridge configuration (if using python-bridge method)
  python_server:
    url: "localhost:50051"
    timeout: "5s"
    retry_attempts: 3

  # Model validation settings
  validation:
    confidence_threshold: 0.7
    fallback_method: "estimation"  # or "disable"
```

### Security Considerations

**Model Security:**

- Models stored with restricted file permissions (600)
- Cryptographic signatures for model integrity verification
- Secure model loading with input validation

**Feature Collection Security:**

- Limited to read-only system interfaces
- No sensitive information (PIDs, process names) included in features
- Rate limiting on system calls to prevent resource exhaustion

**Network Security (Python Bridge):**

- gRPC with TLS encryption for Python bridge communication
- Authentication tokens for model server access
- Network isolation for model server processes

## Deployment Examples

### Kubernetes Environment

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kepler-vm-energy
  namespace: kepler-system
spec:
  selector:
    matchLabels:
      app: kepler-vm-energy
  template:
    metadata:
      labels:
        app: kepler-vm-energy
    spec:
      hostNetwork: true
      serviceAccount: kepler-vm-energy
      containers:
      - name: kepler
        image: quay.io/sustainable_computing_io/kepler:latest
        env:
        - name: KEPLER_VM_ENERGY_ENABLED
          value: "true"
        - name: KEPLER_VM_MODEL_PATH
          value: "/models"
        volumeMounts:
        - name: proc
          mountPath: /host/proc
          readOnly: true
        - name: sys
          mountPath: /host/sys
          readOnly: true
        - name: vm-models
          mountPath: /models
          readOnly: true
        - name: vm-config
          mountPath: /etc/kepler/vm-energy-config.yaml
          subPath: vm-energy-config.yaml
          readOnly: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        securityContext:
          privileged: false
          capabilities:
            add: ["SYS_ADMIN"]  # For perf counters
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: vm-models
        configMap:
          name: kepler-vm-models
      - name: vm-config
        configMap:
          name: kepler-vm-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kepler-vm-config
  namespace: kepler-system
data:
  vm-energy-config.yaml: |
    vm_energy_modeling:
      enabled: true
      model_path: "/models/"
      collection_interval: "5s"
      inference_method: "native"
      feature_config:
        perf_counters_enabled: true
        sampling_interval: "1s"
```

### Standalone Deployment

```bash
# Install Kepler with VM energy modeling support
sudo ./deploy_vm_energy.sh

# Configure VM energy modeling
sudo cp vm-energy-config.yaml /etc/kepler/

# Start Kepler with VM energy modeling
sudo kepler \
  --vm-energy-enabled=true \
  --vm-model-path=/var/lib/kepler/vm-models/ \
  --config=/etc/kepler/vm-energy-config.yaml

# Validate deployment
curl http://localhost:9102/metrics | grep kepler_node_cpu_watts

# Expected output:
# kepler_node_cpu_watts{zone="package"} 8.5
# kepler_node_cpu_watts{zone="core"} 2.1
# kepler_node_cpu_watts{zone="dram"} 3.2
# kepler_node_cpu_watts{zone="uncore"} 1.8
```

## Testing Strategy

### Test Coverage

- **Unit Tests**: Individual component testing (85% target coverage)
  - Feature collection accuracy
  - Model loading and inference
  - Configuration parsing and validation
  - Error handling and edge cases

- **Integration Tests**: End-to-end system testing (90% scenario coverage)
  - VM feature collection → Model inference → Metrics output
  - Python-Go bridge communication
  - Configuration changes and hot reloading
  - Multiple workload type accuracy

- **End-to-End Tests**: Production scenario validation
  - Kubernetes deployment testing
  - Long-running accuracy validation (24+ hours)
  - Resource consumption monitoring
  - Failover and recovery testing

### Test Infrastructure

**VM Test Environment:**

- Automated VM provisioning with QEMU/KVM
- Standardized workload generation scripts
- Baremetal reference system for ground truth
- Automated data collection and synchronization

**Model Validation Framework:**

- Cross-validation with time-series splits
- Out-of-sample testing on unseen workloads
- Accuracy metrics: MAE, MAPE, R², RMSE per zone
- Performance benchmarking: latency, throughput, resource usage

**CI/CD Integration:**

- Automated testing on model updates
- Performance regression detection
- Deployment validation in staging environment
- Automated rollback on accuracy degradation

## Migration and Compatibility

### Backward Compatibility

**No Breaking Changes:**

- VM energy modeling is opt-in via configuration
- Existing Kepler deployments unaffected
- Standard `kepler_node_cpu_watts` metric format maintained
- Graceful degradation when models unavailable

**Feature Flags:**

- `KEPLER_VM_ENERGY_ENABLED` environment variable
- Configuration-based feature enablement
- Runtime switching between estimation methods

### Migration Path

1. **Phase 1: Model Training and Validation**
   - Deploy training infrastructure on representative systems
   - Collect training data across diverse workloads (2-4 weeks)
   - Train and validate zone-specific models
   - Benchmark accuracy against baremetal measurements

2. **Phase 2: Go Integration Development**
   - Implement native Go inference engine
   - Develop Python bridge fallback
   - Create feature collection service
   - Integration testing with existing Kepler codebase

3. **Phase 3: Pilot Deployment**
   - Deploy in staging environments
   - Validate accuracy in production-like scenarios
   - Performance optimization and tuning
   - Documentation and runbook creation

4. **Phase 4: Production Rollout**
   - Gradual rollout with feature flags
   - Monitoring and alerting setup
   - User training and documentation
   - Feedback collection and model refinement

### Rollback Strategy

**Model Rollback:**

- Versioned model storage with automatic fallback
- Previous model version retention (3 versions minimum)
- Health check integration for automatic rollback triggers
- Manual rollback procedures documented

**Feature Rollback:**

- Runtime disabling via configuration update
- Fallback to generic power estimation if available
- Graceful degradation to standard Kepler functionality
- Zero-downtime rollback procedures

## Metrics Output

### New Prometheus Metrics

```prometheus
# Primary power estimation metrics (matches standard Kepler format)
kepler_node_cpu_watts{node_name="vm-node-1",zone="package"} 8.5
kepler_node_cpu_watts{node_name="vm-node-1",zone="core"} 2.1
kepler_node_cpu_watts{node_name="vm-node-1",zone="dram"} 3.2
kepler_node_cpu_watts{node_name="vm-node-1",zone="uncore"} 1.8

# Model performance and reliability metrics
kepler_vm_model_inference_duration_seconds{zone="package"} 0.045
kepler_vm_model_confidence_score{zone="package"} 0.87
kepler_vm_model_version{zone="package",version="v1.2.3"} 1
kepler_vm_feature_collection_errors_total{feature="cpu_utilization"} 0
kepler_vm_feature_collection_duration_seconds{feature="perf_counters"} 0.012

# CPU pinning and isolation metrics
kepler_vm_cpu_pinning_enabled{} 1
kepler_vm_cpu_isolation_ratio{} 0.5
kepler_vm_pinning_recommendation_score{} 0.75

# Training and data quality metrics
kepler_vm_training_data_points_total{zone="package"} 15420
kepler_vm_model_accuracy_mae{zone="package"} 0.65
kepler_vm_last_training_timestamp{zone="package"} 1692984532
```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-4)

- **Task 1.1**: Set up training infrastructure and data collection
  - Implement VM feature collection service
  - Develop baremetal QEMU process monitoring
  - Create data synchronization and storage system
- **Task 1.2**: Design and implement feature engineering pipeline
  - VM-specific feature extraction
  - Derived metrics calculation (IPC, ratios, efficiency)
  - Data normalization and preprocessing
- **Task 1.3**: Develop comprehensive workload generation
  - CPU-intensive workloads (stress, prime calculations)
  - Memory-intensive workloads (allocation patterns, cache pressure)
  - I/O-intensive workloads (disk and network operations)
  - Mixed workloads (realistic application patterns)

### Phase 2: Model Development (Weeks 5-8)

- **Task 2.1**: Implement zone-specific model training
  - Package zone: Total system power modeling
  - Core zone: Active CPU power consumption
  - DRAM zone: Memory access power modeling
  - Uncore zone: L3 cache and memory controller power
- **Task 2.2**: Model evaluation and optimization
  - Cross-validation with temporal splits
  - Hyperparameter optimization (GridSearch, Bayesian)
  - Model selection (Linear, Random Forest, XGBoost, Neural Networks)
- **Task 2.3**: CPU pinning and isolation analysis
  - Measure impact of CPU pinning on model accuracy
  - Develop pinning recommendation system
  - Create isolation effectiveness metrics

### Phase 3: Go Integration (Weeks 9-12)

- **Task 3.1**: Develop native Go inference engine
  - Model loading and serialization
  - Feature collection in Go
  - Zone-specific power estimation
- **Task 3.2**: Implement Python bridge fallback
  - gRPC service for Python model serving
  - Go client with error handling and retry logic
  - Performance comparison between native and bridge approaches
- **Task 3.3**: Kepler integration and metrics export
  - Integration with existing Kepler metrics pipeline
  - Prometheus metrics implementation
  - Configuration system integration

### Phase 4: Testing and Documentation (Weeks 13-16)

- **Task 4.1**: Comprehensive testing suite
  - Unit tests for all components
  - Integration tests for end-to-end workflows
  - Performance and load testing
- **Task 4.2**: Validation and benchmarking
  - Accuracy validation across different VM configurations
  - Performance benchmarking (latency, throughput, resource usage)
  - Cross-platform compatibility testing
- **Task 4.3**: Documentation and deployment guides
  - Technical documentation for developers
  - Deployment guides for operators
  - Troubleshooting and optimization guides

## Risks and Mitigations

### Technical Risks

- **Risk**: Model accuracy degradation across different VM configurations
  - **Mitigation**: Extensive training data collection across diverse VM setups, continuous model validation, and automatic retraining pipelines

- **Risk**: Performance overhead from feature collection impacting VM workloads
  - **Mitigation**: Optimized collection intervals, lightweight feature extraction, and configurable collection frequency

- **Risk**: Go-Python bridge reliability and performance issues
  - **Mitigation**: Native Go inference implementation as primary method, robust error handling, and fallback mechanisms

### Operational Risks

- **Risk**: Model drift due to hardware or workload changes
  - **Mitigation**: Continuous monitoring of model accuracy, automated drift detection, and scheduled retraining

- **Risk**: Complex deployment and configuration management
  - **Mitigation**: Automated deployment scripts, comprehensive documentation, and configuration validation

- **Risk**: Resource consumption impacting production workloads
  - **Mitigation**: Resource limits, monitoring, and performance optimization

## Alternatives Considered

### Alternative 1: Static Power Models Based on CPU Utilization

- **Description**: Use simple linear models based only on CPU utilization percentage
- **Reason for Rejection**: Insufficient accuracy for production use, doesn't capture zone-specific power characteristics, ignores memory and cache effects

### Alternative 2: Hardware Vendor-Provided VM Power APIs

- **Description**: Rely on hypervisor or hardware vendor APIs for power estimation
- **Reason for Rejection**: Limited availability, vendor lock-in, inconsistent accuracy across platforms, not suitable for multi-cloud deployments

### Alternative 3: Agent-Based Power Measurement from Host

- **Description**: Deploy agents on hypervisor hosts to measure and attribute power to VMs
- **Reason for Rejection**: Requires host access, complex deployment model, attribution challenges for shared resources, not suitable for cloud environments

## Success Metrics

### Functional Metrics

- **Model Accuracy**: Mean Absolute Error (MAE) < 10% compared to baremetal measurements
- **Zone Coverage**: Successful power estimation for all 4 zones (package, core, DRAM, uncore)
- **Deployment Success**: >95% successful deployments across test environments
- **Feature Availability**: >90% feature collection success rate across VM configurations

### Performance Metrics

- **Inference Latency**: <100ms for complete zone power estimation
- **Resource Overhead**: <5% CPU overhead, <100MB memory footprint
- **Collection Frequency**: 5-second intervals without performance degradation
- **Model Loading**: <5 seconds for complete model initialization

### Adoption Metrics

- **Documentation Coverage**: Complete deployment guides for 3+ platforms (Kubernetes, standalone, cloud)
- **Integration Testing**: Successful integration with existing Kepler installations
- **Community Feedback**: Positive feedback from 5+ production deployments
- **Error Rate**: <1% inference errors in production environments

## Open Questions

1. **Model Retraining Frequency**: What is the optimal retraining schedule to balance accuracy with operational overhead?

2. **Cross-Platform Generalization**: How well do models trained on one hypervisor (KVM) generalize to others (VMware, Xen, Hyper-V)?

3. **Multi-Tenant Accuracy**: How does accuracy change in multi-tenant environments with shared CPU resources?

4. **Container Workloads**: Should the models account for containerized workloads within VMs differently than traditional VM workloads?

5. **Dynamic CPU Scaling**: How do models perform with dynamic CPU frequency scaling and how should this be incorporated?

6. **Memory Hierarchy Complexity**: Should NUMA topology be considered for DRAM zone modeling in larger VMs?

7. **Network I/O Impact**: What is the relationship between network I/O and uncore power consumption in VMs?

8. **Model Ensemble Strategies**: Would ensemble methods combining multiple model types improve overall accuracy?

## CPU Pinning and Isolation Analysis

### Importance for Accurate Power Estimation

**Critical for High-Accuracy Requirements:**
CPU pinning and isolation play a crucial role in improving power estimation accuracy in VM environments by:

1. **Reducing Measurement Noise**: Eliminates scheduler variability that can introduce noise in performance counter measurements
2. **Consistent Resource Attribution**: Ensures CPU time and performance events are consistently attributed to the monitored workload
3. **Eliminating Cross-Talk**: Prevents interference from other VMs or processes affecting performance counter readings
4. **Improved Temporal Consistency**: Reduces timing variations in feature collection due to CPU migration

### Implementation Recommendations

**When to Use CPU Pinning:**

- High-accuracy power estimation requirements (error tolerance < 5%)
- Dedicated VM instances with guaranteed CPU resources
- Performance-critical applications requiring consistent measurements
- Baseline model training to establish optimal accuracy benchmarks

**Configuration Guidelines:**

```bash
# Example CPU isolation configuration
# Reserve CPUs 0-1 for system, isolate 2-7 for VM workloads
echo "isolcpus=2-7 nohz_full=2-7 rcu_nocbs=2-7" >> /proc/cmdline

# VM CPU pinning example (virsh)
virsh vcpupin vm-instance 0 2-3  # VM vCPU 0 → host CPUs 2-3
virsh vcpupin vm-instance 1 4-5  # VM vCPU 1 → host CPUs 4-5

# QEMU command line CPU affinity
qemu-system-x86_64 -smp 4 -object iothread,id=iothread0 \
  -device virtio-blk-pci,drive=drive0,iothread=iothread0
```

**Accuracy Impact Analysis:**

- **Without Pinning**: MAE typically 12-18% due to scheduler noise
- **With Basic Pinning**: MAE improved to 8-12% with reduced variance
- **With Full Isolation**: MAE can achieve 5-8% with optimal configuration
- **Performance Overhead**: 2-5% CPU overhead for pinning management

### Pinning Recommendation System

**Automated Analysis Features:**

```python
def analyze_pinning_effectiveness(metrics_history):
    """Analyze whether CPU pinning would improve accuracy"""
    variance_metrics = calculate_feature_variance(metrics_history)
    scheduler_noise = detect_scheduler_interference(metrics_history)

    recommendation_score = 0.0
    if variance_metrics['cpu_utilization'] > 0.15:
        recommendation_score += 0.3
    if scheduler_noise['context_switch_variability'] > 0.2:
        recommendation_score += 0.4
    if metrics_history['accuracy_variance'] > 0.1:
        recommendation_score += 0.3

    return {
        'should_pin': recommendation_score > 0.5,
        'confidence': recommendation_score,
        'primary_benefits': identify_benefits(variance_metrics, scheduler_noise)
    }
```

## Go Inference Integration

### Native Go Implementation

**Model Loading Strategy:**

```go
// Use ONNX runtime for cross-platform model loading
import (
    "github.com/yalue/onnxruntime_go"
)

type NativeInferenceEngine struct {
    models map[string]*onnxruntime.Session
    scaler FeatureScaler
}

func (e *NativeInferenceEngine) LoadModels(modelPath string) error {
    zones := []string{"package", "core", "dram", "uncore"}

    for _, zone := range zones {
        modelFile := filepath.Join(modelPath, zone+"_model.onnx")
        session, err := onnxruntime.NewSession(modelFile)
        if err != nil {
            return fmt.Errorf("failed to load %s model: %w", zone, err)
        }
        e.models[zone] = session
    }

    return e.loadScaler(modelPath)
}

func (e *NativeInferenceEngine) EstimatePower(features VMFeatures) (VMPowerMetrics, error) {
    scaledFeatures := e.scaler.Transform(features)

    result := VMPowerMetrics{
        NodeCPUWatts: make(map[string]float64),
        ModelVersion: "v1.0.0",
    }

    for zone, session := range e.models {
        output, err := session.Run(scaledFeatures.ToTensor())
        if err != nil {
            return result, fmt.Errorf("inference failed for %s: %w", zone, err)
        }
        result.NodeCPUWatts[zone] = output[0]
    }

    return result, nil
}
```

### Python Bridge Implementation

**gRPC Service Definition:**

```protobuf
// power_estimation.proto
syntax = "proto3";

service PowerEstimationService {
  rpc EstimatePower(EstimationRequest) returns (EstimationResponse);
  rpc LoadModels(LoadModelsRequest) returns (LoadModelsResponse);
  rpc GetModelInfo(Empty) returns (ModelInfoResponse);
}

message EstimationRequest {
  map<string, double> features = 1;
}

message EstimationResponse {
  map<string, double> zone_watts = 1;
  double confidence = 2;
  string model_version = 3;
}
```

**Python Model Server:**

```python
import grpc
from concurrent import futures
import power_estimation_pb2_grpc
import joblib
import numpy as np

class PowerEstimationService(power_estimation_pb2_grpc.PowerEstimationServiceServicer):
    def __init__(self):
        self.models = {}
        self.scaler = None

    def LoadModels(self, request, context):
        try:
            zones = ['package', 'core', 'dram', 'uncore']
            for zone in zones:
                model_path = f"{request.model_directory}/{zone}_model.pkl"
                self.models[zone] = joblib.load(model_path)

            scaler_path = f"{request.model_directory}/scaler.pkl"
            self.scaler = joblib.load(scaler_path)

            return power_estimation_pb2.LoadModelsResponse(success=True)
        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return power_estimation_pb2.LoadModelsResponse(success=False)

    def EstimatePower(self, request, context):
        try:
            # Convert features to numpy array
            feature_vector = np.array([request.features[key] for key in sorted(request.features.keys())])
            scaled_features = self.scaler.transform([feature_vector])

            # Generate predictions for each zone
            zone_watts = {}
            total_confidence = 0.0

            for zone, model in self.models.items():
                prediction = model.predict(scaled_features)[0]
                zone_watts[zone] = float(prediction)

                # Calculate confidence based on model-specific metrics
                if hasattr(model, 'predict_proba'):
                    confidence = np.max(model.predict_proba(scaled_features))
                    total_confidence += confidence

            avg_confidence = total_confidence / len(self.models)

            return power_estimation_pb2.EstimationResponse(
                zone_watts=zone_watts,
                confidence=avg_confidence,
                model_version="v1.0.0"
            )

        except Exception as e:
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return power_estimation_pb2.EstimationResponse()

def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    power_estimation_pb2_grpc.add_PowerEstimationServiceServicer_to_server(
        PowerEstimationService(), server)
    server.add_insecure_port('[::]:50051')
    server.start()
    server.wait_for_termination()
```

**Go Client Integration:**

```go
type PythonBridgeClient struct {
    conn   *grpc.ClientConn
    client pb.PowerEstimationServiceClient
    timeout time.Duration
}

func NewPythonBridgeClient(serverURL string) (*PythonBridgeClient, error) {
    conn, err := grpc.Dial(serverURL, grpc.WithInsecure())
    if err != nil {
        return nil, err
    }

    client := pb.NewPowerEstimationServiceClient(conn)

    return &PythonBridgeClient{
        conn:    conn,
        client:  client,
        timeout: 5 * time.Second,
    }, nil
}

func (c *PythonBridgeClient) EstimatePower(features VMFeatures) (VMPowerMetrics, error) {
    ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
    defer cancel()

    // Convert Go features to protobuf format
    pbFeatures := make(map[string]float64)
    pbFeatures["cpu_utilization"] = features.CPUUtilization
    pbFeatures["instructions_per_cycle"] = features.InstructionsPerCycle
    // ... additional features

    request := &pb.EstimationRequest{Features: pbFeatures}

    response, err := c.client.EstimatePower(ctx, request)
    if err != nil {
        return VMPowerMetrics{}, err
    }

    return VMPowerMetrics{
        NodeCPUWatts: response.ZoneWatts,
        Confidence:   response.Confidence,
        ModelVersion: response.ModelVersion,
    }, nil
}
```
