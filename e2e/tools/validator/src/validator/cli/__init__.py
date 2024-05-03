# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0
import click
import os
from validator.__about__ import __version__
from validator.stresser.stresser import ( 
    run_script,
)
from validator.prom_query_validator.prom_query_validator import (
    PromMetricsValidator, deltas_func, percentage_err
)
import yaml
from typing import NamedTuple


import statistics


#TODO: decide where to keep the scripts 


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

class Metal(NamedTuple):
    vm: VM

class Prometheus(NamedTuple):
    url: str

class Config(NamedTuple):
    remote: Remote
    metal: Metal
    prometheus: Prometheus

    def __repr__(self):
        return f"<Config {self.remote}@{self.prometheus}>"

pass_config = click.make_pass_decorator(Config)


def load_config(config_file: str) -> Config:
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
    remote = Remote(
        host=remote_config['host'],
        port=remote_config.get('port', 22),
        user=remote_config.get('username', 'fedora'),
        password=remote_config.get('password', None),
        pkey=os.path.expanduser(remote_config.get('pkey', '~/.ssh/id_rsa')),
    )

    metal_config = config['metal']
    vm_config = metal_config['vm']
    vm = VM( pid=vm_config['pid'],)
    metal = Metal(vm=vm)

    prometheus_config = config['prometheus']
    prometheus = Prometheus(
        url=prometheus_config['url'],
    )

    return Config(
        remote=remote, 
        metal=metal, 
        prometheus=prometheus,
    )


@click.group(
    context_settings={"help_option_names": ["-h", "--help"]}, 
    invoke_without_command=False,
)
@click.version_option(version=__version__, prog_name="validator")
@click.option(
   "--config-file", "-f", default="validator.yaml",
    type=click.Path(exists=True),
)
@click.pass_context
def validator(ctx: click.Context, config_file: str):
    ctx.obj = load_config(config_file)


@validator.command()
@click.option(
    "--script-path", "-s", 
    default="scripts/stressor.sh", 
    type=str,
)
@pass_config
def stress(cfg: Config, script_path: str):
    PROM_QUERIES = {
        "vm_process_joules_total": {"name": "kepler_process_package_joules_total", "base_labels": {"job": "metal", "pid": "2093543"}},
        "platform_joules_vm": {"name": "kepler_node_platform_joules_total", "base_labels": {"job": "vm"}},
        # "platform_joules_vm_bm" : "kepler_vm_platform_joules_total{job='metal'}"
    }

    remote = cfg.remote

    start_time, end_time  = run_script(
        host=remote.host,
        port=remote.port,
        username=remote.user,
        password=remote.password,
        pkey_path=remote.pkey,
        script_path=script_path,
    )

    # from prometheus_api_client.utils import parse_datetime
    # start_time=parse_datetime("2024-04-12 16:27:20.254648")
    # end_time = parse_datetime("2024-04-12 16:28:00.466223")
    click.echo(f"start_time: {start_time}, end_time: {end_time}")


    # TODO: clean up
    expected_query_config = PROM_QUERIES["vm_process_joules_total"]
    expected_query_modified_labels = expected_query_config["base_labels"].copy()
    expected_query_modified_labels["pid"] = str(cfg.metal.vm.pid)
    #expected_query = "kepler_process_package_joules_total{pid='2093543', job='metal'}"
    actual_query_config = PROM_QUERIES["platform_joules_vm"]

    prom_validator = PromMetricsValidator(
        endpoint=cfg.prometheus.url,
        disable_ssl=True,
    )
    validator_data, validated_data = prom_validator.compare_metrics(
        start_time=start_time, 
        end_time=end_time, 
        expected_query=expected_query_config["name"],
        expected_query_labels=expected_query_modified_labels,
        actual_query=actual_query_config["name"],
        actual_query_labels=actual_query_config["base_labels"]
    )
    print(validator_data)
    # NOTE: calc
    percentage_error = percentage_err(validator_data, validated_data)
    absolute_error = deltas_func(validator_data, validated_data)
    mae = statistics.mean(absolute_error)
    mape = statistics.mean(percentage_error)

    # TODO: print what the values mean
    click.secho("Validation results during stress test:")
    click.secho(f"Absolute Errors during stress test: {absolute_error}", fg='green')
    click.secho(f"Absolute Percentage Errors during stress test: {percentage_error}", fg='green')
    click.secho(f"Mean Absolute Error (MAE) during stress test: {mae}", fg="blue")
    click.secho(f"Mean Absolute Percentage Error (MAPE) during stress test: {mape}", fg="blue")


