# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import os
import yaml
from typing import NamedTuple

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

class Prometheus(NamedTuple):
    url: str
    interval: str
    step: str

class Validator(NamedTuple):
    remote: Remote
    metal: Metal
    prometheus: Prometheus
    query_path: str

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
    with open(config_file, 'r') as file:
        config = yaml.safe_load(file)

    remote_config = config['remote']
    # NOTE: set default path to pkey if password is not set

    pkey = remote_config.get('pkey', '')
    if pkey != '':
        pkey=os.path.expanduser(pkey)

    # NOTE: set default path to pkey if password is not set
    if remote_config.get('password') is None:
        pkey=os.path.expanduser(pkey or '~/.ssh/id_rsa')

    remote = Remote(
        host=remote_config['host'],
        port=remote_config.get('port', 22),
        user=remote_config.get('username', 'fedora'),
        password=remote_config.get('password', ''),
        pkey=pkey
    )

    metal_config = config['metal']
    vm_config = metal_config['vm']
    pid = vm_config.get('pid', 0)
    vm_name = vm_config.get('name', '')
    vm = VM(pid=pid, name=vm_name)
    metal = Metal(vm=vm)

    prometheus_config = config['prometheus']
    prometheus = Prometheus(
        url=prometheus_config['url'],
        interval=prometheus_config.get('interval', '12s'),
        step=prometheus_config.get('step', '3s')
    )

    query_path = config.get('query_path', 'query.json' )

    return Validator(
        remote=remote, 
        metal=metal, 
        prometheus=prometheus,
        query_path=query_path
    )
