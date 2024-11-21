# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import os
from typing import NamedTuple

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
