# Setup

<!--toc:start-->
- [Setup](#setup)
  - [Baremetal](#baremetal)
    - [Usage](#usage)
  - [Virtual Machine](#virtual-machine)
<!--toc:end-->

In order to validate models, here is the recommended setup assuming the
development box is a Baremetal.

- Create a Virtual Machine

- Ensure the CPU is pinned to the VM. E.g. if you are using libvirt, you can
   use `vcpupin` as follows

   ```text
   virsh vcpupin <vm_name> <cpu_number> <core-number>
   ```

## Baremetal

The `metal` compose target should be run on a metal / development machine. This
target deploys the following

- kepler - built from source with process metrics enabled
- scaphandre: as a target to compare kepler metrics against
- prometheus: that scrapes - metal kepler, scaphandre, and a host identified as
  `vm`
- grafana with a dashboard for comparing metal process metrics (qemu) against vm

### Usage

To allow access to the VM from the Prometheus container running on the host,
`virt-net` must be created using the following command:

- Check the Virtual Bridge Interface:

   ```sh
   ip addr show virbr0
   ```

   Look for `inet` in the output and use that for the subnet in the next command

- Create the Docker Network:

Use the `inet` address from the previous step to create the Docker Network with
`macvlan` driver. Replace `<subnet>` with appropriate value.

```sh
docker network create --driver=macvlan --subnet=<subnet>/24 -o parent=virbr0 virt-net
```

- Start the Services:

Navigate to appropriate directory, set the `VM_IP` environment variable, and bring
up the services:

```sh
cd manifests/compose/validation/metal
export VM_IP=192.168....
docker compose up
```

## Virtual Machine

- `vm` compose target should be run on a Virtual Machine. This target deploys
   `kepler` with `estimator` and `model-server` version: 0.7.7 Usage:

```sh
git clone https://github.com/sustainable-computing-io/kepler
cd manifests/compose/validation/vm
docker compose up
```

Ensure the setup works by `curl http://<vm-ip>:9100/metrics` from the Baremetal
machine.
