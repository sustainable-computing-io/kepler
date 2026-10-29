// SPDX-FileCopyrightText: 2025 The Kepler Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"fmt"
	"github.com/sustainable-computing-io/kepler/config"
	"github.com/sustainable-computing-io/kepler/internal/device"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu"
	"github.com/sustainable-computing-io/kepler/internal/device/gpu/nvidia"
	"github.com/sustainable-computing-io/kepler/internal/k8s/pod"
	"github.com/sustainable-computing-io/kepler/internal/monitor"
	"github.com/sustainable-computing-io/kepler/internal/platform/redfish"
	"github.com/sustainable-computing-io/kepler/internal/resource"
	"github.com/sustainable-computing-io/kepler/internal/service"
	"k8s.io/utils/ptr"
	"log/slog"
)

func compose(logger *slog.Logger, cfg *config.Config) (*Engine, error) {
	logger.Debug("Creating all services")
	cpuPowerMeter, err := device.CreateCPUMeter(logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create CPU power meter: %w", err)
	}

	// GPU meters are optional - returns empty slice if not available
	gpuMeters := createGPUMeters(logger, cfg)

	// Inject configured idle power into GPU meters that support it
	if cfg.Experimental != nil && cfg.Experimental.GPU.IdlePower > 0 {
		for _, m := range gpuMeters {
			if c, ok := m.(gpu.IdlePowerConfigurable); ok {
				c.SetIdlePower(cfg.Experimental.GPU.IdlePower)
				logger.Info("configured GPU idle power",
					"watts", cfg.Experimental.GPU.IdlePower)
			}
		}
	}

	// Inject configured DCGM endpoint for MIG power attribution
	if cfg.Experimental != nil && cfg.Experimental.GPU.DCGMEndpoint != "" {
		for _, m := range gpuMeters {
			if c, ok := m.(gpu.DCGMEndpointConfigurable); ok {
				c.SetDCGMEndpoint(cfg.Experimental.GPU.DCGMEndpoint)
			}
		}
	}

	e := &Engine{logger: logger, Config: cfg}
	// Hardware factories return initialized meters. Do not initialize them twice.
	e.initialized = append(e.initialized, cpuPowerMeter)
	for _, m := range gpuMeters {
		e.initialized = append(e.initialized, m)
	}
	var services []service.Service

	var podInformer pod.Informer
	if *cfg.Kube.Enabled {
		podInformer = createPodInformer(cfg, logger)
		services = append(services, podInformer)
	}
	resourceOpts := []resource.OptionFn{
		resource.WithLogger(logger),
		resource.WithProcFSPath(cfg.Host.ProcFS),
		resource.WithPodInformer(podInformer),
	}
	if podInformer != nil && cfg.Kube.PodName != "" {
		// "/" only means kepler's own cgroup in a private cgroup namespace; in the host
		// namespace it is the host root cgroup, and its processes are not kepler's
		if private, err := resource.InPrivateCgroupNamespace(resource.SelfCgroupFile); err != nil {
			logger.Warn("Not attributing kepler's own processes to its container", "error", err)
		} else if !private {
			logger.Info("Kepler runs in the host cgroup namespace, its processes are attributed through their cgroup path")
		} else {
			resourceOpts = append(resourceOpts, resource.WithSelfContainer(resource.SelfContainer{
				Namespace:     cfg.Kube.PodNamespace,
				PodName:       cfg.Kube.PodName,
				ContainerName: cfg.Kube.ContainerName,
			}))
		}
	}
	resourceInformer, err := resource.NewInformer(resourceOpts...)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to create resource informer: %w", err), shutdownServices(e.initialized))
	}

	// Build monitor options
	pmOpts := []monitor.OptionFn{
		monitor.WithLogger(logger),
		monitor.WithResourceInformer(resourceInformer),
		monitor.WithInterval(cfg.Monitor.Interval),
		monitor.WithMaxStaleness(cfg.Monitor.Staleness),
		monitor.WithMaxTerminated(cfg.Monitor.MaxTerminated),
		monitor.WithMinTerminatedEnergyThreshold(monitor.Energy(cfg.Monitor.MinTerminatedEnergyThreshold) * monitor.Joule),
	}
	if len(gpuMeters) > 0 {
		pmOpts = append(pmOpts, monitor.WithGPUPowerMeters(gpuMeters))
	}

	pm := monitor.NewPowerMonitor(cpuPowerMeter, pmOpts...)

	// Create Redfish service if enabled (experimental feature)

	e.Monitor = pm
	services = append(services, resourceInformer, pm)
	if cfg.IsFeatureEnabled(config.ExperimentalRedfishFeature) {
		rs, err := createRedfishService(logger, cfg)
		if err != nil {
			logger.Warn("Redfish service unavailable, continuing without platform power monitoring", "error", err)
		} else {
			services = append(services, rs)
			e.Platform = rs
		}
	}
	e.services = services
	return e, nil
}

func createPodInformer(cfg *config.Config, logger *slog.Logger) pod.Informer {
	if cfg.Kube.PodInformer.Mode == "apiserver" {
		logger.Info("using API server pod informer")
		return pod.NewInformer(
			pod.WithLogger(logger),
			pod.WithKubeConfig(cfg.Kube.Config),
			pod.WithNodeName(cfg.Kube.Node),
		)
	}

	// Default: kubelet-based informer
	logger.Info("using kubelet pod informer",
		"pollInterval", cfg.Kube.PodInformer.PollInterval)
	return pod.NewKubeletInformer(
		pod.WithLogger(logger),
		pod.WithNodeName(cfg.Kube.Node),
		pod.WithKubeConfig(cfg.Kube.Config),
		pod.WithPollInterval(cfg.Kube.PodInformer.PollInterval),
	)
}

func createRedfishService(logger *slog.Logger, cfg *config.Config) (*redfish.Service, error) {
	return redfish.NewService(cfg.Experimental.Platform.Redfish, logger, redfish.WithStaleness(cfg.Monitor.Staleness))
}

// createGPUMeters discovers and initializes GPU power meters for all vendors.
// Uses the registry pattern to support multiple GPU vendors (NVIDIA, AMD, Intel).
// Returns empty slice if GPU is not enabled or no GPUs are available (soft-fail).
func createGPUMeters(logger *slog.Logger, cfg *config.Config) []gpu.GPUPowerMeter {
	if fake := cfg.Dev.FakeGPUMeter; ptr.Deref(fake.Enabled, false) {
		return createFakeGPUMeter(logger, fake.DeviceCount, fake.SharingMode)
	}

	if !cfg.IsFeatureEnabled(config.ExperimentalGPUFeature) {
		return nil
	}

	// DiscoverAll probes all registered GPU backends and returns initialized meters
	meters := gpu.DiscoverAll(logger)
	if len(meters) == 0 {
		logger.Info("no GPUs discovered on this node")
		return nil
	}

	// Log all discovered GPUs
	for _, m := range meters {
		logger.Info("GPU discovered",
			"vendor", m.Vendor(),
			"devices", len(m.Devices()))
	}

	return meters
}

// createFakeGPUMeter creates a GPU meter backed by FakeNVMLBackend for
// development and testing. Not for production use.
func createFakeGPUMeter(logger *slog.Logger, deviceCount int, sharingMode string) []gpu.GPUPowerMeter {
	if deviceCount <= 0 {
		logger.Error("invalid fake GPU device count", "deviceCount", deviceCount)
		return nil
	}

	var mode nvidia.ComputeMode
	switch sharingMode {
	case "exclusive":
		mode = nvidia.ComputeModeExclusiveProcess
	case "time-slicing":
		mode = nvidia.ComputeModeDefault
	default:
		logger.Error("invalid fake GPU sharing mode", "sharingMode", sharingMode)
		return nil
	}

	logger.Warn("using FAKE NVML backend (dev/testing mode)",
		"deviceCount", deviceCount,
		"sharingMode", sharingMode)

	devices := make([]*nvidia.FakeNVMLDevice, deviceCount)
	for i := range devices {
		devices[i] = &nvidia.FakeNVMLDevice{
			Idx:         i,
			DeviceUUID:  fmt.Sprintf("FAKE-GPU-%04d", i),
			DeviceName:  "Fake NVIDIA GPU",
			DevicePower: 225 * device.Watt,
			Mode:        mode,
		}
	}
	// TODO: Add fake GPU processes when e2e workloads can supply stable PIDs.

	backend := nvidia.NewFakeNVMLBackend(devices...)
	collector, err := nvidia.NewGPUPowerCollector(logger, nvidia.WithNVMLBackend(backend))
	if err != nil {
		logger.Error("failed to create GPU collector with fake backend", "error", err)
		return nil
	}

	// Init must be called before returning, matching the real path (gpu.Discover
	// calls Init before returning the meter). This ensures Devices() is populated
	// when the monitor logs GPU meter info during its own Init.
	if err := collector.Init(); err != nil {
		logger.Error("failed to initialize fake GPU collector", "error", err)
		return nil
	}

	return []gpu.GPUPowerMeter{collector}
}
