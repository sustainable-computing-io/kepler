# CPU Model
[cpus.yaml](./cpus.yaml) lists CPU information includes: `Core`, `Uarch`, `Family`, `Model` and `Stepping`.

`Core` refers to the CPU core name.

`Family`, `Model` and `Stepping` are information included in /proc/cpuinfo, could be fetched by Golang libraries such as "github.com/klauspost/cpuid/v2".

`Uarch` refers to the CPU microarchitecture.

Help needed for any vendors' CPU Models data missing in this file when you test Kepler on your platform.

Please feel free to raise issue in Kepler for your case.

[power_model.csv](./legacy/power_model.csv) and [power_data.csv](./legacy/power_data.csv) are legacy data files for Kepler CPU power model.
