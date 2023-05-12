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
	// MaxIRQ is the maximum number of IRQs to be monitored
	MaxIRQ = 10
)

var (
	modelServerService = fmt.Sprintf("kepler-model-server.%s.svc.cluster.local", KeplerNamespace)

	EnabledMSR            = false
	EnabledBPFBatchDelete = false

	KernelVersion = float32(0)

	KeplerNamespace              = getConfig("KEPLER_NAMESPACE", defaultNamespace)
	EnabledEBPFCgroupID          = getBoolConfig("ENABLE_EBPF_CGROUPID", true)
	EnabledGPU                   = getBoolConfig("ENABLE_GPU", false)
	EnableProcessMetrics         = getBoolConfig("ENABLE_PROCESS_METRICS", false)
	ExposeHardwareCounterMetrics = getBoolConfig("EXPOSE_HW_COUNTER_METRICS", true)
	ExposeCgroupMetrics          = getBoolConfig("EXPOSE_CGROUP_METRICS", true)
	ExposeKubeletMetrics         = getBoolConfig("EXPOSE_KUBELET_METRICS", true)
	ExposeIRQCounterMetrics      = getBoolConfig("EXPOSE_IRQ_COUNTER_METRICS", true)
	MetricPathKey                = "METRIC_PATH"
	BindAddressKey               = "BIND_ADDRESS"
	CPUArchOverride              = getConfig("CPU_ARCH_OVERRIDE", "")

	EstimatorModel        = getConfig("ESTIMATOR_MODEL", defaultMetricValue)         // auto-select
	EstimatorSelectFilter = getConfig("ESTIMATOR_SELECT_FILTER", defaultMetricValue) // no filter
	CoreUsageMetric       = getConfig("CORE_USAGE_METRIC", CPUInstruction)
	DRAMUsageMetric       = getConfig("DRAM_USAGE_METRIC", CacheMiss)
	UncoreUsageMetric     = getConfig("UNCORE_USAGE_METRIC", defaultMetricValue) // no metric (evenly divided)
	GpuUsageMetric        = getConfig("GPU_USAGE_METRIC", GPUSMUtilization)      // no metric (evenly divided)
	GeneralUsageMetric    = getConfig("GENERAL_USAGE_METRIC", CPUInstruction)    // for uncategorized energy; pkg - core - uncore

	versionRegex = regexp.MustCompile(`^(\d+)\.(\d+).`)

	configPath = "/etc/kepler/kepler.config"

	////////////////////////////////////
	ModelServerEnable   = getBoolConfig("MODEL_SERVER_ENABLE", false)
	ModelServerEndpoint = SetModelServerReqEndpoint()
	// for model config
	modelConfigValues map[string]string
	// model_item
	NodeTotalKey           = "NODE_TOTAL"
	NodeComponentsKey      = "NODE_COMPONENTS"
	ContainerTotalKey      = "CONTAINER_TOTAL"
	ContainerComponentsKey = "CONTAINER_COMPONENTS"
	ProcessTotalKey        = "PROCESS_TOTAL"
	ProcessComponentsKey   = "PROCESS_COMPONENTS"

	//  attribute
	EstimatorEnabledKey = "ESTIMATOR"
	InitModelURLKey     = "INIT_URL"
	FixedModelNameKey   = "MODEL"
	ModelFiltersKey     = "FILTERS"
	////////////////////////////////////

	// KubeConfig is used to start k8s client with the pod running outside the cluster
	KubeConfig = ""
)

func logBoolConfigs() {
	if klog.V(5).Enabled() {
		klog.V(5).Infof("ENABLE_EBPF_CGROUPID: %t", EnabledEBPFCgroupID)
		klog.V(5).Infof("ENABLE_GPU: %t", EnabledGPU)
		klog.V(5).Infof("ENABLE_PROCESS_METRICS: %t", EnableProcessMetrics)
		klog.V(5).Infof("EXPOSE_HW_COUNTER_METRICS: %t", ExposeHardwareCounterMetrics)
		klog.V(5).Infof("EXPOSE_CGROUP_METRICS: %t", ExposeCgroupMetrics)
		klog.V(5).Infof("EXPOSE_KUBELET_METRICS: %t", ExposeKubeletMetrics)
		klog.V(5).Infof("EXPOSE_IRQ_COUNTER_METRICS: %t", ExposeIRQCounterMetrics)
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

func getConfig(configKey, defaultValue string) (result string) {
	result = string([]byte(defaultValue))
	key := string([]byte(configKey))
	configFile := filepath.Join(configPath, key)
	value, err := os.ReadFile(configFile)
	if err == nil {
		result = bytes.NewBuffer(value).String()
	} else {
		strValue, present := os.LookupEnv(key)
		if present {
			result = strValue
		}
	}
	return
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

// InitModelConfigMap initializes map of config from MODEL_CONFIG
func InitModelConfigMap() {
	modelConfigValues = getModelConfigMap()
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

// SetEnabledGPU enables the exposure of gpu metrics
func SetEnabledGPU(enabled bool) {
	// set to true if any config source set it to true
	EnabledGPU = enabled || EnabledGPU
}

// SetKubeConfig set kubeconfig file
func SetKubeConfig(k string) {
	KubeConfig = k
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

func getModelConfigMap() map[string]string {
	configMap := make(map[string]string)
	modelConfigStr := getConfig("MODEL_CONFIG", "")
	lines := strings.Fields(modelConfigStr)
	for _, line := range lines {
		values := strings.Split(line, "=")
		if len(values) == 2 {
			configMap[values[0]] = values[1]
		}
	}
	return configMap
}

func getModelConfigKey(modelItem, attribute string) string {
	return fmt.Sprintf("%s_%s", modelItem, attribute)
}

func GetModelConfig(modelItem string) (useEstimatorSidecar bool, selectedModel, selectFilter, initModelURL string) {
	useEstimatorSidecarStr := modelConfigValues[getModelConfigKey(modelItem, EstimatorEnabledKey)]
	if strings.EqualFold(useEstimatorSidecarStr, "true") {
		useEstimatorSidecar = true
	}
	selectedModel = modelConfigValues[getModelConfigKey(modelItem, FixedModelNameKey)]
	selectFilter = modelConfigValues[getModelConfigKey(modelItem, ModelFiltersKey)]
	initModelURL = modelConfigValues[getModelConfigKey(modelItem, InitModelURLKey)]
	return
}
