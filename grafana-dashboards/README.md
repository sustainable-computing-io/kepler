# Grafana Dashboard
This directory stores pre-generated Grafana dashboard. Due to data format changes, dashboards used prior to PR #112 are in [legacy](./legacy) directory.

# Customerize Dashboard
The metrics used by Pod are:
```
kepler_container_package_joules_total{}
kepler_container_dram_joules_total{}
kepler_container_gpu_joules_total{}
kepler_container_other_host_components_joules_total{}
```
