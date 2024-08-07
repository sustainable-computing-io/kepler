# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

# a python program to get host and VM cpu spec, dram size, number of cpu cores, and return a json output
import subprocess
import typing

from validator import config
from validator.stresser import Remote


class SubprocessError(Exception):
    pass


class CPUSpec(typing.NamedTuple):
    model: str
    cores: str
    threads: str
    sockets: str
    flags: str


def parse_lscpu_output(output: str) -> CPUSpec:
    cpu_spec: dict[str, str] = {}

    for line in output.split("\n"):
        if line:
            key, value = line.split(":", 1)
            v = value.strip()
            if key == "Model name":
                cpu_spec["model"] = v
            elif key == "CPU(s)":
                cpu_spec["cores"] = v
            elif key == "Thread(s) per core":
                cpu_spec["threads"] = v
            elif key == "Socket(s)":
                cpu_spec["sockets"] = v
            elif key == "Flags":
                cpu_spec["flags"] = v

    return CPUSpec(**cpu_spec)


def get_host_cpu_spec() -> CPUSpec:
    # get host cpu spec
    lscpu = subprocess.run(["/usr/bin/lscpu"], stdout=subprocess.PIPE, check=False)
    return parse_lscpu_output(lscpu.stdout.decode())


def get_vm_cpu_spec(vm: Remote) -> CPUSpec:
    lscpu = vm.run("lscpu")
    if lscpu.exit_code != 0:
        msg = "failed to run lscpu on vm"
        raise SubprocessError(msg)

    return parse_lscpu_output(lscpu.stdout)


def get_host_dram_size() -> str:
    # get host dram size
    with open("/proc/meminfo") as meminfo:
        for line in meminfo:
            if "MemTotal" in line:
                return line.split(":")[1].strip()
    return ""


def get_vm_dram_size(vm: Remote) -> str:
    meminfo = vm.run("cat", "/proc/meminfo")
    for line in meminfo.stdout.split("\n"):
        if "MemTotal" in line:
            return line.split(":")[1].strip()

    return ""


class MachineSpec(typing.NamedTuple):
    cpu_spec: CPUSpec
    dram_size: str


def get_host_spec() -> MachineSpec:
    return MachineSpec(get_host_cpu_spec(), get_host_dram_size())


def get_vm_spec(r: config.Remote) -> MachineSpec:
    vm = Remote(r)
    return MachineSpec(get_vm_cpu_spec(vm), get_vm_dram_size(vm))


# class HostReporter:
#     def __init__(self, host: config.Metal) -> None:
#         self.host = host
#
#     def write(self, report: typing.TextIO) -> None:
#         host_cpu_spec = get_host_cpu_spec()
#         host_dram_size = get_host_dram_size()
#
#         # create section header for specs
#         report.write("## Specs\n")
#         report.write("### Host CPU Specs\n")
#         report.write("| Model | Cores | Threads | Sockets | Flags |\n")
#         report.write("|-----------|-----------|-------------|-------------|-----------|\n")
#         report.write(
#             f"| {host_cpu_spec['cpu']['model']} | {host_cpu_spec['cpu']['cores']} | {host_cpu_spec['cpu']['threads']} | {host_cpu_spec['cpu']['sockets']} | ```{host_cpu_spec['cpu']['flags']}``` |\n"
#         )
#         report.write("### Host DRAM Size\n")
#         report.write("| Size |\n")
#         report.write("|------|\n")
#         report.write(f"| {host_dram_size} |\n")
#         report.flush()
#
