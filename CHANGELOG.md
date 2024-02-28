in kepler 0.7 release
- switch to libbpf as default ebpf provider
- base image update decouple GPU driver from kepler image itself
- use kprobe instead of tracepoint for ebpf to obtain context switch information
- add task clock event to ebpf and use it to calculate cpu usage for each process. The event is also exported to prometheus
- add initial NVIDIA DCGM support, this help monitor power consumption by NVIDIA GPU especially MIG instances.
- add new curvefit regressors to predict component power consumption
- add workload pipeline to build container base images on demand
- add ARM64 container image and RPM build support