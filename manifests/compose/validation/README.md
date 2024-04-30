
## Setup

In order to validate models, here is the recommended setup assuming
the development box is a Baremetal.

1. Create a Virtual Machine
2. Ensure the CPU is pinned to the VM. 
		E.g. if you are using libvirt, you can use `vcpupin` as follows

	```
	virsh vcpupin <vm_name> <cpu_number> <core-number>
	```

### Baremetal

The `metal` compose target should be run on a metal / development machine. This 
target deploys the following
  - kepler - built from source with process metrics enabled
  - scaphandre: as a target to compare kepler metrics against
  - prometheus: that scrapes - metal kepler, scaphandre, and a host identified as `vm` 
  - grafana with a dashboard for comparing metal process metrics (qemu) against vm

**usage: **

```sh
cd manifests/compose/validation/metal
export VM_IP=192.168....
docker compose up
```

### Virtual Machine

2. `vm` compose target should be run on a Virtual Machine. 
This target deploys `kepler` with `estimator` and `model-server` version: 0.7.7
Usage: 

```
git clone https://github.com/sustainable-computing-io/kepler
cd manifests/compose/validation/metal
docker compose up
```
Ensure the setup works by `curl http://<vm-ip>:9100/metrics` from the Baremetal 
machine.

