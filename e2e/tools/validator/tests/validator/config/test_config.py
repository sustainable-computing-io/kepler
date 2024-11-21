import os

import pytest

from validator.config import load


@pytest.fixture
def config_file(tmp_path):
    file_path = tmp_path / "config.yaml"

    def write(content):
        with open(file_path, "w+t") as file:
            file.write(content)
        return file_path

    return write


@pytest.fixture
def minimal_config_file(config_file):
    return config_file(
        """
remote:
  host: example.com

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090

stressor:
  total_runtime_seconds: 1200
  curve_type: default
    """
    )


def test_minimal_config_file(minimal_config_file):
    """
    tests that defaults are set correctly

    """
    config = load(minimal_config_file)
    assert config.log_level == "warn"
    assert config.validations_file == "validations.yaml"

    remote = config.remote
    assert remote.host == "example.com"
    assert remote.port == 22
    assert remote.user == "fedora"

    # empty password should automatically set pkey
    assert remote.password == ""
    assert remote.pkey == os.path.expanduser("~/.ssh/id_rsa")

    prometheus = config.prometheus
    assert prometheus.url == "http://localhost:9090"
    assert prometheus.job.metal == "metal"
    assert prometheus.job.vm == "vm"


@pytest.fixture
def stressor_config_file(config_file):
    return config_file(
        """
remote:
  host: example.com

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090

stressor:
  total_runtime_seconds: 1200
  curve_type: default
    """
    )


def test_stressor_config(stressor_config_file):
    config = load(stressor_config_file)
    stressor = config.stressor
    assert stressor.total_runtime_seconds == 1200
    assert stressor.curve_type == "default"


@pytest.fixture
def config_file_use_password(config_file):
    return config_file(
        """
remote:
  host: example.com
  password: supersecret

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090

stressor:
  total_runtime_seconds: 1200
  curve_type: default
"""
    )


def test_config_file_with_password(config_file_use_password):
    config = load(config_file_use_password)
    remote = config.remote
    assert remote.host == "example.com"
    assert remote.port == 22
    assert remote.user == "fedora"
    assert remote.password == "supersecret"
    assert remote.pkey == ""


@pytest.fixture
def config_file_job_override(config_file):
    return config_file(
        """
remote:
  host: example.com
  password: supersecret

metal:
  vm:
    pid: 1337

stressor:
  total_runtime_seconds: 1200
  curve_type: default

prometheus:
  url: http://localhost:9090

  job:
    metal: metal-override
    vm: vm-override
"""
    )


def test_config_file_job_override(config_file_job_override):
    config = load(config_file_job_override)
    prom = config.prometheus
    assert prom.job.metal == "metal-override"
    assert prom.job.vm == "vm-override"


@pytest.fixture
def config_file_password_empty_pkey(config_file):
    return config_file(
        """
remote:
  host: example.com
  password: supersecret
  pkey:

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090

stressor:
  total_runtime_seconds: 1200
  curve_type: default
"""
    )


def test_config_file_password_empty_pkey(config_file_password_empty_pkey):
    config = load(config_file_password_empty_pkey)
    remote = config.remote
    assert remote.host == "example.com"
    assert remote.port == 22
    assert remote.user == "fedora"
    assert remote.password == "supersecret"
    assert remote.pkey is None
