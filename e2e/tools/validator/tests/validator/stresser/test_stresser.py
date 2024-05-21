import pytest
from validator.stresser import Remote
from validator.config import load
import os


@pytest.fixture
def validator_params():
    current_dir = os.path.dirname(__file__)
    parent_dir = os.path.dirname(current_dir)
    config_file = os.path.join(parent_dir, "validator_test.yaml")
    validator = load(config_file)
    return validator


def test_connect_to_vm():
    pass


def test_copy_file_to_vm():
    pass


def test_run_script_on_vm():
    pass
