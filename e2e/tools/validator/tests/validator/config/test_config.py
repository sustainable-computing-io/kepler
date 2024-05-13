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

    yield write

@pytest.fixture
def minimal_config_file(config_file):
    return config_file("""
remote:
  host: example.com

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090
    """)

def test_minimal_config_file(minimal_config_file):
    """
    tests that defaults are set correctly

    """
    config = load(minimal_config_file)
    remote = config.remote
    assert remote.host == "example.com"
    assert remote.port == 22
    assert remote.user == "fedora"

    # empty password should automatically set pkey
    assert remote.password == ""
    assert remote.pkey == os.path.expanduser("~/.ssh/id_rsa")

@pytest.fixture
def config_file_use_password(config_file):
    return config_file("""
remote:
  host: example.com
  password: supersecret

metal:
  vm:
    pid: 1337

prometheus:
  url: http://localhost:9090
""")


def test_config_file_with_password(config_file_use_password):
    config = load(config_file_use_password)
    remote = config.remote
    assert remote.host == "example.com"
    assert remote.port == 22
    assert remote.user == "fedora"
    assert remote.password == "supersecret"
    assert remote.pkey == ""
