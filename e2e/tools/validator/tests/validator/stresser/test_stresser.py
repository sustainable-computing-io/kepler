import pytest

from validator import config, stresser


@pytest.fixture
def remote_params():
    return config.Remote(host="192.168.122.51", port=22, user="whisper", password=None, pkey=None)


@pytest.mark.skip(reason="Test requires certain preconditions.")
def test_run_script_on_vm(remote_params):
    remote = stresser.Remote(remote_params)
    script_result = remote.run_script("scripts/stressor.sh")
    start_time = script_result.start_time
    end_time = script_result.end_time
    assert start_time < end_time
