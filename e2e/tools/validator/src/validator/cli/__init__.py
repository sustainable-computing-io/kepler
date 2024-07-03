# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import datetime
import json
import logging
import os
import subprocess
import typing

import click

from validator import config
from validator.__about__ import __version__
from validator.cli import options
from validator.prometheus import Comparator, PrometheusClient, Series
from validator.specs import Reporter as SpecReporter
from validator.stresser import Remote, ScriptResult
from validator.validations import Loader, QueryTemplate, Validation

logger = logging.getLogger(__name__)
pass_config = click.make_pass_decorator(config.Validator)


class Report(typing.NamedTuple):
    path: str
    results_dir: str
    file: typing.TextIO

    def write(self, text: str) -> None:
        self.file.write(text)

    def close(self) -> None:
        self.file.close()

    def flush(self) -> None:
        self.file.flush()

    def h1(self, text: str) -> None:
        self.write(f"# {text}\n\n")

    def h2(self, text: str) -> None:
        self.write(f"## {text}\n\n")

    def h3(self, text: str) -> None:
        self.write(f"### {text}\n\n")

    def h4(self, text: str) -> None:
        self.write(f"#### {text}\n\n")

    def code(self, text: str) -> None:
        self.write(f"```\n{text}\n```\n")


def new_report(dir: str) -> Report:
    # run git describe command and get the output as the report name
    git_describe = subprocess.run(["git", "describe", "--tag"], stdout=subprocess.PIPE, check=False)
    tag = git_describe.stdout.decode().strip()

    results_dir = os.path.join(dir, f"validator-{tag}")
    os.makedirs(results_dir, exist_ok=True)

    path = os.path.join(dir, f"report-{tag}.md")
    file = open(path, "w")

    r = Report(path, results_dir, file)
    r.h1(tag)

    return r


def dump_query_result(raw_results_dir: str, query: QueryTemplate, series: Series):
    out_file = f"{query.metric_name}--{query.mode}.json"
    with open(os.path.join(raw_results_dir, out_file), "w") as f:
        f.write(
            json.dumps(
                {
                    "query": query.one_line,
                    "values": series.values,
                }
            )
        )


@click.group(
    context_settings={"help_option_names": ["-h", "--help"]},
    invoke_without_command=False,
)
@click.version_option(version=__version__, prog_name="validator")
@click.option(
    "--log-level",
    "-l",
    type=click.Choice(["debug", "info", "warn", "error", "config"]),
    default="config",
    required=False,
)
@click.option(
    "--config-file",
    "-f",
    default="validator.yaml",
    type=click.Path(exists=True),
    show_default=True,
)
@click.pass_context
def validator(ctx: click.Context, config_file: str, log_level: str):
    cfg = config.load(config_file)
    log_level = cfg.log_level if log_level == "config" else log_level
    try:
        level = getattr(logging, log_level.upper())
    except AttributeError:
        print(f"Invalid log level: {cfg.log_level}; setting to debug")
        level = logging.DEBUG

    logging.basicConfig(level=level)
    ctx.obj = cfg


@validator.command()
@click.option(
    "--script-path",
    "-s",
    default="./scripts/stressor.sh",
    type=click.Path(exists=True),
    show_default=True,
)
@click.option(
    "--report-dir",
    "-o",
    default="/tmp",
    type=click.Path(exists=True, dir_okay=True, writable=True),
    show_default=True,
)
@pass_config
def stress(cfg: config.Validator, script_path: str, report_dir: str):
    # run git describe command and get the output as the report name
    click.secho("  * Generating report file and dir", fg="green")
    report = new_report(report_dir)
    click.secho(f"\treport: {report.path}", fg="bright_green")
    click.secho(f"\tresults dir: {report.results_dir}", fg="bright_green")

    report_kepler_build_info(report, cfg.prometheus)

    click.secho("  * Generating spec report ...", fg="green")
    sr = SpecReporter(cfg.metal, cfg.remote)
    sr.write(report.file)

    click.secho("  * Running stress test ...", fg="green")
    remote = Remote(cfg.remote)
    stress_test = remote.run_script(script_path)

    generate_validation_report(report, cfg, stress_test)


@validator.command()
@click.option("--start", "-s", type=options.DateTime(), required=True)
@click.option("--end", "-e", type=options.DateTime(), required=True)
@click.option(
    "--report-dir",
    "-o",
    default="/tmp",
    type=click.Path(exists=True),
)
@pass_config
def gen_report(cfg: config.Validator, start: datetime.datetime, end: datetime.datetime, report_dir: str):
    """
    Run only validation based on previously stress test
    """
    report = new_report(report_dir)
    report_kepler_build_info(report, cfg.prometheus)
    generate_spec_report(report, cfg)
    result = ScriptResult(start, end)
    generate_validation_report(report, cfg, result)


def generate_spec_report(report: Report, cfg: config.Validator):
    click.secho("  * Generating spec report ...", fg="green")
    sr = SpecReporter(cfg.metal, cfg.remote)
    sr.write(report.file)


def report_kepler_build_info(report: Report, prom_config: config.Prometheus):
    prom = PrometheusClient(prom_config)
    build_info = prom.kepler_build_info()

    click.secho("\n  * Build Info", fg="green")
    report.h2("Build Info")
    for bi in build_info:
        click.secho(f"    - {bi}", fg="cyan")
        report.write(f"  * `{bi}`\n")

    click.echo()
    report.flush()


def generate_validation_report(report: Report, cfg: config.Validator, test: ScriptResult):
    start_time, end_time = test.start_time, test.end_time

    click.secho("  * Generating validation report", fg="green")
    click.secho(f"   - started at: {start_time}", fg="green")
    click.secho(f"   - ended   at: {end_time}", fg="green")
    click.secho(f"   - duration  : {end_time - start_time}", fg="green")

    report.h2("Validation Results")
    report.write(f"   * Started At: `{start_time}`\n")
    report.write(f"   * Ended   At: `{end_time}`\n")
    report.write(f"   * Duration  : `{end_time - start_time}`\n\n")

    click.secho("\nRun validations", fg="green")
    prom = PrometheusClient(cfg.prometheus)
    comparator = Comparator(prom)
    validations = Loader(cfg).load()
    for v in validations:
        report_validation_results(report, v, comparator, start_time, end_time)
    report.close()

    click.secho("Report Generated ", fg="bright_green")
    click.secho(f"  time range: {start_time} - {end_time}: ({end_time - start_time}) ", fg="bright_green")
    click.secho(f"  report: {report.path} ", fg="bright_green")
    click.secho(f"  dir   : {report.results_dir} ", fg="bright_green")


def report_validation_results(
    report: Report,
    v: Validation,
    comparator: Comparator,
    start_time: datetime.datetime,
    end_time: datetime.datetime,
):
    click.secho(f"\t * {v.name}", fg="bright_blue")

    report.h3(f"Validate - {v.name}")
    report.write(f"  * expected:  `{v.expected.one_line}`\n")
    report.write(f"  * actual:  `{v.actual.one_line}`\n")

    try:
        res = comparator.compare(
            start_time,
            end_time,
            v.expected.promql,
            v.actual.promql,
        )
        click.secho(f"\t    MSE : {res.mse}", fg="bright_blue")
        click.secho(f"\t    MAPE: {res.mape}\n", fg="bright_blue")

        report.h4("Errors")
        report.write(f"  * MSE: {res.mse}\n")
        report.write(f"  * MAPE: {res.mape}\n")
        report.flush()

        dump_query_result(report.results_dir, v.expected, res.expected_series)
        dump_query_result(report.results_dir, v.actual, res.actual_series)
    except Exception as e:
        click.secho(f"\t    {v.name} failed: {e} ", fg="red")
        click.secho(f"\t    Error: {e} ", fg="yellow")

        report.h4("Unexpected Error")
        report.code(str(e))
        report.flush()
