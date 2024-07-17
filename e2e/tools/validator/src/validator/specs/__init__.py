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


def parse_lscpu_output(output: str):
    cpu_spec: dict[str, dict[str, str]] = {}
    cpu_spec["cpu"] = {}
    cpu_spec["cpu"]["model"] = ""
    cpu_spec["cpu"]["cores"] = ""
    cpu_spec["cpu"]["threads"] = ""
    cpu_spec["cpu"]["sockets"] = ""
    cpu_spec["cpu"]["flags"] = ""

    for line in output.split("\n"):
        if line:
            key, value = line.split(":", 1)
            if key == "Model name":
                cpu_spec["cpu"]["model"] = value.strip()
            elif key == "CPU(s)":
                cpu_spec["cpu"]["cores"] = value.strip()
            elif key == "Thread(s) per core":
                cpu_spec["cpu"]["threads"] = value.strip()
            elif key == "Socket(s)":
                cpu_spec["cpu"]["sockets"] = value.strip()
            elif key == "Flags":
                cpu_spec["cpu"]["flags"] = value.strip()
    return cpu_spec


def get_host_cpu_spec():
    # get host cpu spec
    host_cpu_spec = {}
    lscpu = subprocess.run(["lscpu"], stdout=subprocess.PIPE, check=False)
    if lscpu.stdout:
        host_cpu_spec = parse_lscpu_output(lscpu.stdout.decode())
    return host_cpu_spec


def get_vm_cpu_spec(vm: Remote):
    lscpu = vm.run("lscpu")
    if lscpu.exit_code != 0:
        raise SubprocessError("failed to run lscpu on vm")

    return parse_lscpu_output(lscpu.stdout)


def get_host_dram_size():
    # get host dram size
    dram_size = ""
    meminfo = open("/proc/meminfo")
    for line in meminfo:
        if "MemTotal" in line:
            dram_size = line.split(":")[1].strip()
    return dram_size


def get_vm_dram_size(vm: Remote):
    meminfo = vm.run("cat", "/proc/meminfo")
    vm_dram_size = ""
    for line in meminfo.stdout.split("\n"):
        if "MemTotal" in line:
            vm_dram_size = line.split(":")[1].strip()

    return vm_dram_size


class Reporter:
    def __init__(self, host: config.Metal, vm: config.Remote) -> None:
        self.host = host
        self.vm = vm

    def write(self, report: typing.TextIO) -> None:
        host_cpu_spec = get_host_cpu_spec()
        host_dram_size = get_host_dram_size()

        remote = Remote(self.vm)
        vm_cpu_spec = get_vm_cpu_spec(remote)
        vm_dram_size = get_vm_dram_size(remote)

        # create section header for specs
        report.write("## Specs\n")
        report.write("### Host CPU Specs\n")
        report.write("| Model | Cores | Threads | Sockets | Flags |\n")
        report.write("|-----------|-----------|-------------|-------------|-----------|\n")
        report.write(
            f"| {host_cpu_spec['cpu']['model']} | {host_cpu_spec['cpu']['cores']} | {host_cpu_spec['cpu']['threads']} | {host_cpu_spec['cpu']['sockets']} | ```{host_cpu_spec['cpu']['flags']}``` |\n"
        )
        report.write("### VM CPU Specs\n")
        report.write("| Model | Cores | Threads | Sockets | Flags |\n")
        report.write("|-----------|-----------|-------------|-------------|-----------|\n")
        report.write(
            f"| {vm_cpu_spec['cpu']['model']} | {vm_cpu_spec['cpu']['cores']} | {vm_cpu_spec['cpu']['threads']} | {vm_cpu_spec['cpu']['sockets']} | ```{vm_cpu_spec['cpu']['flags']}``` |\n"
        )
        report.write("### Host DRAM Size\n")
        report.write("| Size |\n")
        report.write("|------|\n")
        report.write(f"| {host_dram_size} |\n")
        report.write("### VM DRAM Size\n")
        report.write("| Size |\n")
        report.write("|------|\n")
        report.write(f"| {vm_dram_size} |\n")
        report.write("\n")
        report.flush()
