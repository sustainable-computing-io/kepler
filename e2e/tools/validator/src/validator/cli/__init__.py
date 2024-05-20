# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import click
from validator.__about__ import __version__
from validator.stresser import ( Remote )

from validator.prometheus import MetricsValidator

from validator.cases import TestCases

from validator.config import (
    Validator, load
)

pass_config = click.make_pass_decorator(Validator)

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
    ctx.obj = load(config_file)


@validator.command()
@click.option(
    "--script-path", "-s", 
    default="scripts/stressor.sh", 
    type=str,
)
@pass_config
def stress(cfg: Validator, script_path: str):
    # PROM_QUERIES = {
    #     "vm_process_joules_total": {"name": "rate(kepler_process_package_joules_total)", "base_labels": {"job": "metal", "pid": "2093543"}},
    #     "platform_joules_vm": {"name": "kepler_node_platform_joules_total", "base_labels": {"job": "vm"}},
    #     # "platform_joules_vm_bm" : "kepler_vm_platform_joules_total{job='metal'}"
    # }

    remote = Remote(cfg.remote)
    result  = remote.run_script(script_path=script_path)

    # from prometheus_api_client.utils import parse_datetime
    # start_time=parse_datetime("2024-04-12 16:27:20.254648")
    # end_time = parse_datetime("2024-04-12 16:28:00.466223")
    click.echo(f"start_time: {result.start_time}, end_time: {result.end_time}")

    # TODO: clean up
    # expected_query_config = PROM_QUERIES["vm_process_joules_total"]
    # expected_query_modified_labels = expected_query_config["base_labels"].copy()
    # expected_query_modified_labels["pid"] = str(cfg.metal.vm.pid)
    #expected_query = "kepler_process_package_joules_total{pid='2093543', job='metal'}"
    #actual_query_config = PROM_QUERIES["platform_joules_vm"]


    # expected_data, actual_data = compare_metrics(
    #     endpoint=cfg.prometheus.url,
    #     disable_ssl=True,
    #     start_time=result.start_time, 
    #     end_time=result.end_time, 
    #     expected_query=expected_query_config["name"],
    #     expected_query_labels=expected_query_modified_labels,
    #     actual_query=actual_query_config["name"],
    #     actual_query_labels=actual_query_config["base_labels"]
    # )
    # # NOTE: calc
    # percentage_error = absolute_percentage_error(expected_data, actual_data)
    # error = absolute_error(expected_data, actual_data)
    # mae = mean_absolute_error(expected_data, actual_data)
    # mape = mean_absolute_percentage_error(expected_data, actual_data)

    test_cases = TestCases(cfg.metal.vm, cfg.prometheus)
    metrics_validator = MetricsValidator(cfg.prometheus)
    test_case_result = test_cases.load_test_cases()
    click.secho("Validation results during stress test:")
    for test_case in test_case_result.test_cases:
        expected_query = test_case.expected_query
        actual_query = test_case.actual_query
        metrics_res = metrics_validator.compare_metrics(result.start_time, 
                                                        result.end_time, 
                                                        expected_query, 
                                                        actual_query)

        click.secho(f"Expected Query Name: {expected_query}", fg='bright_yellow')
        click.secho(f"Actual Query Name: {actual_query}", fg='bright_yellow')      
        click.secho(f"Absolute Errors during stress test: {metrics_res.ae}", fg='green')
        click.secho(f"Absolute Percentage Errors during stress test: {metrics_res.ape}", fg='green')
        click.secho(f"Mean Absolute Error (MAE) during stress test: {metrics_res.mae}", fg="red")
        click.secho(f"Mean Absolute Percentage Error (MAPE) during stress test: {metrics_res.mape}", fg="red")
        click.secho(f"Mean Squared Error (MSE) during stress test: {metrics_res.rmse}", fg="blue")
        click.secho("---------------------------------------------------", fg="cyan")


