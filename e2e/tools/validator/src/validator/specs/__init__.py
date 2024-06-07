# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

# a python program to get host and VM cpu spec, dram size, number of cpu cores, and return a json output
import json
import os
import subprocess
import sys
import re

def parse_lscpu_output(output: str):
    cpu_spec = {}
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
    lscpu = subprocess.run(["lscpu"], stdout=subprocess.PIPE)
    if lscpu.stdout:
        host_cpu_spec = parse_lscpu_output(lscpu.stdout.decode())
    return host_cpu_spec

def get_vm_cpu_spec(login: str = "root", vm_addr: str = "my-vm", key_path: str = "/tmp/vm_ssh_key"):
    vm_cpu_spec = {}
    # run ssh command to get the cpu spec of the VM
    ssh = subprocess.run(["ssh", "-i", key_path, login + "@" + vm_addr, "lscpu"], stdout=subprocess.PIPE)
    if ssh.stdout:
        vm_cpu_spec = parse_lscpu_output(ssh.stdout.decode())
    return vm_cpu_spec

def get_host_dram_size():
    # get host dram size
    dram_size = ""
    meminfo = open("/proc/meminfo", "r")
    for line in meminfo:
        if "MemTotal" in line:
            dram_size = line.split(":")[1].strip()
    return dram_size

def get_vm_dram_size(login: str = "root", vm_addr: str = "my-vm", key_path: str = "/tmp/vm_ssh_key"):
    # get vm dram size
    vm_dram_size = ""
    ssh = subprocess.run(["ssh", "-i", key_path, login + "@" + vm_addr, "cat /proc/meminfo"], stdout=subprocess.PIPE)
    if ssh.stdout:
        for line in ssh.stdout.decode().split("\n"):
            if "MemTotal" in line:
                vm_dram_size = line.split(":")[1].strip()
    return vm_dram_size