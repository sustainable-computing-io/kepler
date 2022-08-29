# Grafana Dashboard
This directory stores pre-generated Grafana dashboard. Due to data format changes, dashboards used prior to PR #112 are in [legacy](./legacy) directory.

# Customerize Dashboard
The metrics used by node and Pod are:
```
node_energy_stat
node_curr_energy_in_[core|dram|uncore|gpu|pkg|other]_joule
pod_curr_energy_in_[core|dram|uncore|gpu|pkg|other]_joule
pod_total_energy_in_[core|dram|uncore|gpu|pkg|other]_joule
```
