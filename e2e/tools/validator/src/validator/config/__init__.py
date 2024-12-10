# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import os
from typing import NamedTuple, Optional

import yaml


class Remote(NamedTuple):
    host: str
    port: int
    user: str
    password: str
    pkey: str

    def __repr__(self):
        return f"<Remote {self.user}@{self.host}>"


class VM(NamedTuple):
    pid: int
    name: str


class Metal(NamedTuple):
    vm: VM


class PrometheusJob(NamedTuple):
    metal: str
    vm: str


class Prometheus(NamedTuple):
    url: str
    rate_interval: str
    step: str
    job: PrometheusJob


class Stressor(NamedTuple):
    total_runtime_seconds: int
    curve_type: str


class Validator(NamedTuple):
    log_level: str
    remote: Remote
    metal: Metal
    prometheus: Prometheus
    stressor: Stressor
    validations_file: str

    def __repr__(self):
        return f"<Config {self.remote}@{self.prometheus}>"


# consider switching to dataclass to avoid repeated fields
class Local(NamedTuple):
    load_curve: str
    iterations: str
    mount_dir: str


class LocalProcess(NamedTuple):
    isolated_cpu: str
    load_curve: str
    iterations: str
    mount_dir: str


class LocalContainer(NamedTuple):
    isolated_cpu: str
    container_name: str
    load_curve: str
    iterations: str
    mount_dir: str


# class LocalPrometheus(NamedTuple):
#     url: str
#     rate_interval: str
#     step: str
#     job_name: str


class BMValidator(NamedTuple):
    log_level: str
    prometheus: Prometheus
    node: Optional[Local]
    process: Optional[LocalProcess]
    container: Optional[LocalContainer]
    validations_file: str


def bload(config_file: str) -> BMValidator:
    """
    Reads Baremetal YAML configuration file and returns a Config object.

    Args:
        config_file (str): Path to Baremetal YAML configuration file.

    Returns:
        BMValidator: A named tuple containing configuration values for Baremetal Validation.
    """
    with open(config_file) as file:
        config = yaml.safe_load(file)

    log_level = config.get("log_level", "warn")
    prom_config = config["prometheus"]
    if not prom_config:
        prom_config = {}
    prom = Prometheus(
        url=prom_config.get("url", "http://localhost:9090"),
        rate_interval=prom_config.get("rate_interval", "20s"),
        step=prom_config.get("step", "3s"),
        job=PrometheusJob(
            metal=prom_config.get("job", "metal"),
            vm="",
        )
    )
    print(prom)

    default_config = config["config"]
    
    node = None
    if "node" in config:
        node_config = config["node"]
        if not node_config:
            node_config = {}
        node = Local(
            load_curve=node_config.get("load_curve", default_config["load_curve"]),
            iterations=node_config.get("iterations", default_config["iterations"]),
            mount_dir=os.path.expanduser(node_config.get("mount_dir", default_config["mount_dir"]))
        )
    print(node)
    
    process = None
    if "process" in config:
        process_config = config["process"]
        if not process_config:
            process_config = {}
        process = LocalProcess(
            isolated_cpu=process_config.get("isolated_cpu", default_config["isolated_cpu"]),
            load_curve=process_config.get("load_curve", default_config["load_curve"]),
            iterations=process_config.get("iterations", default_config["iterations"]),
            mount_dir=os.path.expanduser(process_config.get("mount_dir", default_config["mount_dir"]))
        )
    print(process)
    
    container = None
    if "container" in config:
        container_config = config["container"]
        if not container_config:
            container_config = {}
        container = LocalContainer(
            isolated_cpu=container_config.get("isolated_cpu", default_config["isolated_cpu"]),
            container_name=container_config.get("container_name", default_config["container_name"]),
            load_curve=container_config.get("load_curve", default_config["load_curve"]),
            iterations=container_config.get("iterations", default_config["iterations"]),
            mount_dir=os.path.expanduser(container_config.get("mount_dir", default_config["mount_dir"]))
        )
    print(container)

    validations_file = config.get("validations_file", "bm_validations.yaml")

    return BMValidator(
        log_level=log_level,
        prometheus=prom,
        node=node,
        process=process,
        container=container,
        validations_file=validations_file
    )

def load(config_file: str) -> Validator:
    """
    Reads the YAML configuration file and returns a Config object.

    Args:
        config_file (str): Path to the YAML configuration file.

    Returns:
        Config: A named tuple containing the configuration values.
    """
    with open(config_file) as file:
        config = yaml.safe_load(file)

    remote_config = config["remote"]
    # NOTE: set default path to pkey if password is not set

    pkey = remote_config.get("pkey", "")
    if pkey:
        pkey = os.path.expanduser(pkey)

    # NOTE: set default path to pkey if password is not set
    if not remote_config.get("password"):
        pkey = os.path.expanduser(pkey or "~/.ssh/id_rsa")

    remote = Remote(
        host=remote_config["host"],
        port=remote_config.get("port", 22),
        user=remote_config.get("username", "fedora"),
        password=remote_config.get("password", ""),
        pkey=pkey,
    )

    metal_config = config["metal"]
    vm_config = metal_config["vm"]
    pid = vm_config.get("pid", 0)
    vm_name = vm_config.get("name", "")
    vm = VM(pid=pid, name=vm_name)
    metal = Metal(vm=vm)

    prom_config = config["prometheus"]
    prom_job = prom_config.get("job", {})
    job = PrometheusJob(
        metal=prom_job.get("metal", "metal"),
        vm=prom_job.get("vm", "vm"),
    )

    prometheus = Prometheus(
        url=prom_config["url"],
        # must be 4 x scrape-interval
        rate_interval=prom_config.get("rate_interval", "20s"),
        step=prom_config.get("step", "3s"),
        job=job,
    )

    stressor_config = config["stressor"]
    if not stressor_config:
        stressor = Stressor(total_runtime_seconds=1200, curve_type="default")
    else:
        stressor = Stressor(
            total_runtime_seconds=stressor_config.get("total_runtime_seconds", 1200),
            curve_type=stressor_config.get("curve_type", "default"),
        )

    validations_file = config.get("validations_file", "validations.yaml")
    log_level = config.get("log_level", "warn")

    return Validator(
        remote=remote,
        metal=metal,
        prometheus=prometheus,
        stressor=stressor,
        validations_file=validations_file,
        log_level=log_level,
    )
