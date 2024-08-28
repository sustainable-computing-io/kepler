/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sustainable-computing-io/kepler/pkg/model/types"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	cGroupIDMinKernelVersion = 4.18

	// If this file is present, cgroups v2 is enabled on that node.
	cGroupV2Path = "/sys/fs/cgroup/cgroup.controllers"
)

type Client interface {
	getUnixName() (unix.Utsname, error)
	getCgroupV2File() string
}

type config struct {
}

var c config

const (
	defaultMetricValue      = ""
	defaultNamespace        = "kepler"
	defaultModelServerPort  = "8100"
	defaultModelRequestPath = "/model"
	defaultMaxLookupRetry   = 500
	// MaxIRQ is the maximum number of IRQs to be monitored
	MaxIRQ = 10

	// defaultSamplePeriodSec is the time in seconds that the reader will wait before reading the metrics again
	defaultSamplePeriodSec = 3
)

var (
	modelServerService = fmt.Sprintf("kepler-model-server.%s.svc.cluster.local", KeplerNamespace)

	KernelVersion = float32(0)

	KeplerNamespace              = getConfig("KEPLER_NAMESPACE", defaultNamespace)
	EnabledEBPFCgroupID          = getBoolConfig("ENABLE_EBPF_CGROUPID", true)
	EnabledGPU                   = getBoolConfig("ENABLE_GPU", false)
	EnabledMSR                   = getBoolConfig("ENABLE_MSR", false)
	EnableProcessStats           = getBoolConfig("ENABLE_PROCESS_METRICS", false)
	ExposeContainerStats         = getBoolConfig("EXPOSE_CONTAINER_METRICS", true)
	ExposeVMStats                = getBoolConfig("EXPOSE_VM_METRICS", true)
	ExposeHardwareCounterMetrics = getBoolConfig("EXPOSE_HW_COUNTER_METRICS", true)
	ExposeIRQCounterMetrics      = getBoolConfig("EXPOSE_IRQ_COUNTER_METRICS", true)
	ExposeBPFMetrics             = getBoolConfig("EXPOSE_BPF_METRICS", true)
	ExposeComponentPower         = getBoolConfig("EXPOSE_COMPONENT_POWER", true)
	ExposeIdlePowerMetrics       = getBoolConfig("EXPOSE_ESTIMATED_IDLE_POWER_METRICS", false)
	MockACPIPowerPath            = getConfig("MOCK_ACPI_POWER_PATH", "")

	MetricPathKey   = "METRIC_PATH"
	BindAddressKey  = "BIND_ADDRESS"
	CPUArchOverride = getConfig("CPU_ARCH_OVERRIDE", "")
	MaxLookupRetry  = getIntConfig("MAX_LOOKUP_RETRY", defaultMaxLookupRetry)
	BPFSampleRate   = getIntConfig("EXPERIMENTAL_BPF_SAMPLE_RATE", 0)

	EstimatorModel        = getConfig("ESTIMATOR_MODEL", defaultMetricValue)         // auto-select
	EstimatorSelectFilter = getConfig("ESTIMATOR_SELECT_FILTER", defaultMetricValue) // no filter
	CoreUsageMetric       = getConfig("CORE_USAGE_METRIC", CPUInstruction)
	DRAMUsageMetric       = getConfig("DRAM_USAGE_METRIC", CacheMiss)
	UncoreUsageMetric     = getConfig("UNCORE_USAGE_METRIC", defaultMetricValue)  // no metric (evenly divided)
	GpuUsageMetric        = getConfig("GPU_USAGE_METRIC", GPUComputeUtilization)  // no metric (evenly divided)
	GeneralUsageMetric    = getConfig("GENERAL_USAGE_METRIC", defaultMetricValue) // for uncategorized energy

	SamplePeriodSec = uint64(getIntConfig("SAMPLE_PERIOD_SEC", defaultSamplePeriodSec))

	versionRegex = regexp.MustCompile(`^(\d+)\.(\d+).`)

	configDir = "/etc/kepler/kepler.config"

	// nvidia dcgm hostengine endpoint
	DCGMHostEngineEndpoint = getConfig("NVIDIA_HOSTENGINE_ENDPOINT", "localhost:5555")

	// redfish cred file path
	redfishCredFilePath           string
	redfishProbeIntervalInSeconds = getConfig("REDFISH_PROBE_INTERVAL_IN_SECONDS", "60")
	redfishSkipSSLVerify          = getBoolConfig("REDFISH_SKIP_SSL_VERIFY", true)

	////////////////////////////////////
	DefaultMachineSpecFilePath = "/etc/kepler/models/machine/spec.json"
	machineSpecFilePath        string
	ModelServerEnable          = getBoolConfig("MODEL_SERVER_ENABLE", false)
	ModelServerEndpoint        = SetModelServerReqEndpoint()
	// for model config
	ModelConfigValues map[string]string
	// model_parameter_prefix
	NodePlatformPowerKey        = "NODE_TOTAL"
	NodeComponentsPowerKey      = "NODE_COMPONENTS"
	ContainerPlatformPowerKey   = "CONTAINER_TOTAL"
	ContainerComponentsPowerKey = "CONTAINER_COMPONENTS"
	ProcessPlatformPowerKey     = "PROCESS_TOTAL"
	ProcessComponentsPowerKey   = "PROCESS_COMPONENTS"

	// model_parameter_attribute
	RatioEnabledKey          = "RATIO" // the default container power model is RATIO but ESTIMATOR or LINEAR_REGRESSION can be used
	EstimatorEnabledKey      = "ESTIMATOR"
	LocalRegressorEnabledKey = "LOCAL_REGRESSOR"
	InitModelURLKey          = "INIT_URL"
	FixedTrainerNameKey      = "TRAINER"
	FixedNodeTypeKey         = "NODE_TYPE"
	ModelFiltersKey          = "FILTERS"
	DefaultTrainerName       = types.LinearRegressionTrainer
	////////////////////////////////////

	// KubeConfig is used to start k8s client with the pod running outside the cluster
	KubeConfig      = ""
	EnableAPIServer = false

	////////////////////////////////////

	// Libvirt Metadata config
	LibvirtMetadataURI   = getConfig("LIBVIRT_METADATA_URI", "")
	LibvirtMetadataToken = getConfig("LIBVIRT_METADATA_TOKEN", "name")
)

// return local path to power model weight
// e.g., /var/lib/kepler/data/model_weight/acpi_AbsPowerModel.json
func GetDefaultPowerModelURL(modelOutputType, energySource string) string {
	return fmt.Sprintf(`/var/lib/kepler/data/model_weight/%s_%sModel.json`, energySource, modelOutputType)
}

func logBoolConfigs() {
	if klog.V(5).Enabled() {
		klog.V(5).Infof("ENABLE_EBPF_CGROUPID: %t", EnabledEBPFCgroupID)
		klog.V(5).Infof("ENABLE_GPU: %t", EnabledGPU)
		klog.V(5).Infof("ENABLE_PROCESS_METRICS: %t", EnableProcessStats)
		klog.V(5).Infof("EXPOSE_HW_COUNTER_METRICS: %t", ExposeHardwareCounterMetrics)
		klog.V(5).Infof("EXPOSE_IRQ_COUNTER_METRICS: %t", ExposeIRQCounterMetrics)
		klog.V(5).Infof("EXPOSE_BPF_METRICS: %t", ExposeBPFMetrics)
		klog.V(5).Infof("EXPOSE_COMPONENT_POWER: %t", ExposeComponentPower)
		klog.V(5).Infof("EXPOSE_ESTIMATED_IDLE_POWER_METRICS: %t. This only impacts when the power is estimated using pre-prained models. Estimated idle power is meaningful only when Kepler is running on bare-metal or with a single virtual machine (VM) on the node.", ExposeIdlePowerMetrics)
		klog.V(5).Infof("EXPERIMENTAL_BPF_SAMPLE_RATE: %d", BPFSampleRate)
	}
}

func LogConfigs() {
	logBoolConfigs()
}

func getBoolConfig(configKey string, defaultBool bool) bool {
	defaultValue := "false"
	if defaultBool {
		defaultValue = "true"
	}
	return strings.ToLower(getConfig(configKey, defaultValue)) == "true"
}

func getIntConfig(configKey string, defaultInt int) int {
	defaultValue := fmt.Sprintf("%d", defaultInt)
	value, err := strconv.Atoi((getConfig(configKey, defaultValue)))
	if err == nil {
		return value
	}
	return defaultInt
}

// getConfig returns the value of the key by first looking in the environment
// and then in the config file if it exists or else returns the default value.
func getConfig(key, defaultValue string) string {
	// env var takes precedence over config file
	if envValue, exists := os.LookupEnv(key); exists {
		return envValue
	}

	// return config file value if there is one
	configFile := filepath.Join(configDir, key)
	if value, err := os.ReadFile(configFile); err == nil {
		return strings.TrimSpace(bytes.NewBuffer(value).String())
	}

	return defaultValue
}

func SetRedfishCredFilePath(credFilePath string) {
	redfishCredFilePath = credFilePath
}

func GetRedfishCredFilePath() string {
	return redfishCredFilePath
}

func SetRedfishProbeIntervalInSeconds(interval string) {
	redfishProbeIntervalInSeconds = interval
}

func GetRedfishProbeIntervalInSeconds() int {
	// convert string "redfishProbeIntervalInSeconds" to int
	probeInterval, err := strconv.Atoi(redfishProbeIntervalInSeconds)
	if err != nil {
		klog.Warning("failed to convert redfishProbeIntervalInSeconds to int", err)
		return 60
	}
	return probeInterval
}

func SetRedfishSkipSSLVerify(skipSSLVerify bool) {
	redfishSkipSSLVerify = skipSSLVerify
}

func GetRedfishSkipSSLVerify() bool {
	return redfishSkipSSLVerify
}

func SetModelServerReqEndpoint() (modelServerReqEndpoint string) {
	modelServerURL := getConfig("MODEL_SERVER_URL", modelServerService)
	if modelServerURL == modelServerService {
		modelServerPort := getConfig("MODEL_SERVER_PORT", defaultModelServerPort)
		modelServerPort = strings.TrimSuffix(modelServerPort, "\n") // trim \n for kustomized manifest
		modelServerURL = fmt.Sprintf("http://%s:%s", modelServerURL, modelServerPort)
	}
	modelReqPath := getConfig("MODEL_SERVER_MODEL_REQ_PATH", defaultModelRequestPath)
	modelServerReqEndpoint = modelServerURL + modelReqPath
	return
}

func GetMockACPIPowerPath() string {
	return MockACPIPowerPath
}

func SetMachineSpecFilePath(specFilePath string) {
	machineSpecFilePath = specFilePath
}

// GetMachineSpec initializes a map of MachineSpecValues from MACHINE_SPEC
func GetMachineSpec() *MachineSpec {
	if machineSpecFilePath != "" {
		if spec, err := readMachineSpec(machineSpecFilePath); err == nil {
			return spec
		} else {
			klog.Warningf("failed to read spec from %s: %v, use default machine spec", machineSpecFilePath, err)
		}
	}
	return getDefaultMachineSpec()
}

// InitModelConfigMap initializes map of config from MODEL_CONFIG
func InitModelConfigMap() {
	ModelConfigValues = GetModelConfigMap()
}

// SetEnabledEBPFCgroupID enables the eBPF code to collect cgroup id if the system has kernel version > 4.18
func SetEnabledEBPFCgroupID(enabled bool) {
	// set to false if any config source set it to false
	enabled = enabled && EnabledEBPFCgroupID
	klog.Infoln("using gCgroup ID in the BPF program:", enabled)
	KernelVersion = getKernelVersion(c)
	klog.Infoln("kernel version:", KernelVersion)
	if (enabled) && (KernelVersion >= cGroupIDMinKernelVersion) && (isCGroupV2(c)) {
		EnabledEBPFCgroupID = true
	} else {
		EnabledEBPFCgroupID = false
	}
}

// SetEnabledHardwareCounterMetrics enables the exposure of hardware counter metrics
func SetEnabledHardwareCounterMetrics(enabled bool) {
	// set to false is any config source set it to false
	ExposeHardwareCounterMetrics = enabled && ExposeHardwareCounterMetrics
}

// SetEnabledIdlePower allows enabling idle power exposure in Kepler's metrics. When direct power metrics access is available,
// idle power exposure is automatic. With pre-trained power models, awareness of implications is crucial.
// Estimated idle power is useful for bare-metal or single VM setups. In VM environments, accurately distributing idle power is tough due
// to unknown co-running VMs. Wrong division results in significant accuracy errors, duplicatiing the host idle power across all VMs.
// Container pre-trained models focus on dynamic power. Estimating idle power in limited information scenarios (like VMs) is complex.
// Idle power prediction is limited to bare-metal or single VM setups.
// Know the number of running VMs becomes crucial for achieving a fair distribution of idle power, particularly when following the GHG (Greenhouse Gas) protocol.
func SetEnabledIdlePower(enabled bool) {
	// set to true is any config source set it to true or if system power metrics are available
	ExposeIdlePowerMetrics = enabled || ExposeIdlePowerMetrics
	if ExposeIdlePowerMetrics {
		klog.Infoln("The Idle power will be exposed. Are you running on Baremetal or using single VM per node?")
	}
}

// IsIdlePowerEnabled always return true if Kepler has access to system power metrics.
// However, if pre-trained power models are being used, Kepler should only expose metrics if the user is aware of the implications.
func IsIdlePowerEnabled() bool {
	return ExposeIdlePowerMetrics
}

// IsExposeProcessStatsEnabled returns false if process metrics are disabled to minimize overhead in the Kepler standalone mode.
func IsExposeProcessStatsEnabled() bool {
	return EnableProcessStats
}

// IsExposeContainerStatsEnabled returns false if container metrics are disabled to minimize overhead in the Kepler standalone mode.
func IsExposeContainerStatsEnabled() bool {
	return ExposeContainerStats
}

// IsExposeVMStatsEnabled returns false if VM metrics are disabled to minimize overhead.
func IsExposeVMStatsEnabled() bool {
	return ExposeVMStats
}

// IsExposeBPFMetricsEnabled returns false if BPF Metrics metrics are disabled to minimize overhead.
func IsExposeBPFMetricsEnabled() bool {
	return ExposeBPFMetrics
}

// IsExposeComponentPowerEnabled returns false if component power metrics are disabled to minimize overhead.
func IsExposeComponentPowerEnabled() bool {
	return ExposeComponentPower
}

// SetEnabledGPU enables the exposure of gpu metrics
func SetEnabledGPU(enabled bool) {
	// set to true if any config source set it to true
	EnabledGPU = enabled || EnabledGPU
}

// SetEnabledMSR enables the exposure of MSR metrics
func SetEnabledMSR(enabled bool) {
	// set to true if any config source set it to true
	EnabledMSR = enabled || EnabledMSR
}

// SetKubeConfig set kubeconfig file
func SetKubeConfig(k string) {
	KubeConfig = k
}

// SetEnableAPIServer enables Kepler to watch apiserver
func SetEnableAPIServer(enabled bool) {
	EnableAPIServer = enabled
}

func (c config) getUnixName() (unix.Utsname, error) {
	var utsname unix.Utsname
	err := unix.Uname(&utsname)
	return utsname, err
}

func (c config) getCgroupV2File() string {
	return cGroupV2Path
}

func getKernelVersion(c Client) float32 {
	utsname, err := c.getUnixName()
	if err != nil {
		klog.V(4).Infoln("Failed to parse unix name")
		return -1
	}
	// per https://github.com/google/cadvisor/blob/master/machine/info.go#L164
	kv := utsname.Release[:bytes.IndexByte(utsname.Release[:], 0)]

	versionParts := versionRegex.FindStringSubmatch(string(kv))
	if len(versionParts) < 2 {
		klog.V(1).Infof("got invalid release version %q (expected format '4.3-1 or 4.3.2-1')\n", kv)
		return -1
	}
	major, err := strconv.Atoi(versionParts[1])
	if err != nil {
		klog.V(1).Infof("got invalid release major version %q\n", major)
		return -1
	}

	minor, err := strconv.Atoi(versionParts[2])
	if err != nil {
		klog.V(1).Infof("got invalid release minor version %q\n", minor)
		return -1
	}

	v, err := strconv.ParseFloat(fmt.Sprintf("%d.%d", major, minor), 32)
	if err != nil {
		klog.V(1).Infof("parse %d.%d got issue: %v", major, minor, err)
		return -1
	}
	return float32(v)
}

func isCGroupV2(c Client) bool {
	_, err := os.Stat(c.getCgroupV2File())
	return !os.IsNotExist(err)
}

// Get cgroup version, return 1 or 2
func GetCGroupVersion() int {
	if isCGroupV2(c) {
		return 2
	} else {
		return 1
	}
}

func SetEstimatorConfig(modelName, selectFilter string) {
	EstimatorModel = modelName
	EstimatorSelectFilter = selectFilter
}

func SetModelServerEndpoint(serverEndpoint string) {
	ModelServerEndpoint = serverEndpoint
}

func GetMetricPath(cmdSet string) string {
	return getConfig(MetricPathKey, cmdSet)
}

func GetBindAddress(cmdSet string) string {
	return getConfig(BindAddressKey, cmdSet)
}

func GetModelConfigMap() map[string]string {
	configMap := make(map[string]string)
	modelConfigStr := getConfig("MODEL_CONFIG", "")
	lines := strings.Fields(modelConfigStr)
	for _, line := range lines {
		values := strings.Split(line, "=")
		if len(values) == 2 {
			k := strings.TrimSpace(values[0])
			v := strings.TrimSpace(values[1])
			configMap[k] = v
		}
	}
	return configMap
}

func SetGpuUsageMetric(metric string) {
	GpuUsageMetric = metric
}

func GetLibvirtMetadataURI() string {
	return LibvirtMetadataURI
}

func GetLibvirtMetadataToken() string {
	return LibvirtMetadataToken
}
