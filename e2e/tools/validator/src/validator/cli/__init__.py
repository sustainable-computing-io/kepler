# SPDX-FileCopyrightText: 2024-present Sunil Thaha <sthaha@redhat.com>
#
# SPDX-License-Identifier: APACHE-2.0

import click
import subprocess
from validator.__about__ import __version__
from validator.stresser import ( Remote )

from validator.prometheus import MetricsValidator

from validator.cases import Cases

from validator.config import (
    Validator, load
)

from validator.specs import (
    get_host_cpu_spec, get_vm_cpu_spec, get_host_dram_size, get_vm_dram_size
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
    # run git describe command and get the output as the report name
    tag = ""
    git_describe = subprocess.run(["git", "describe", "--tag"], stdout=subprocess.PIPE)
    if git_describe.stdout:
        tag = git_describe.stdout.decode().strip()
    host_cpu_spec = get_host_cpu_spec()
    vm_cpu_spec = get_vm_cpu_spec()
    host_dram_size = get_host_dram_size()
    vm_dram_size = get_vm_dram_size()
    # save all the print result into a markdown file as a table
    report = open(f"/tmp/report-{tag}.md", "w")
    # create section header for specs
    report.write(f"# {tag}\n")
    report.write("## Specs\n")
    report.write("### Host CPU Specs\n")
    report.write("| Model | Cores | Threads | Sockets | Flags |\n")
    report.write("|-----------|-----------|-------------|-------------|-----------|\n")
    report.write(f"| {host_cpu_spec['cpu']['model']} | {host_cpu_spec['cpu']['cores']} | {host_cpu_spec['cpu']['threads']} | {host_cpu_spec['cpu']['sockets']} | ```{host_cpu_spec['cpu']['flags']}``` |\n")
    report.write("### VM CPU Specs\n")
    report.write("| Model | Cores | Threads | Sockets | Flags |\n")
    report.write("|-----------|-----------|-------------|-------------|-----------|\n")
    report.write(f"| {vm_cpu_spec['cpu']['model']} | {vm_cpu_spec['cpu']['cores']} | {vm_cpu_spec['cpu']['threads']} | {vm_cpu_spec['cpu']['sockets']} | ```{vm_cpu_spec['cpu']['flags']}``` |\n")
    report.write("### Host DRAM Size\n")
    report.write("| Size |\n")
    report.write("|------|\n")
    report.write(f"| {host_dram_size} |\n")
    report.write("### VM DRAM Size\n")
    report.write("| Size |\n")
    report.write("|------|\n")
    report.write(f"| {vm_dram_size} |\n")
    report.write("\n")
    # create section header for validation results
    report.write("## Validation Results\n")
    report.flush()
    remote = Remote(cfg.remote)
    result  = remote.run_script(script_path=script_path)
    click.echo(f"start_time: {result.start_time}, end_time: {result.end_time}")
    test_cases = Cases(
        vm = cfg.metal.vm, metal_job_name = cfg.metal.metal_job_name, vm_job_name = cfg.metal.vm_job_name,
        prom = cfg.prometheus, query_path = cfg.query_path
        )
    metrics_validator = MetricsValidator(cfg.prometheus)
    test_case_result = test_cases.load_test_cases()
    click.secho("Validation results during stress test:")
    for test_case in test_case_result.test_cases:

        query = test_case.refined_query

        print(f"start_time: {result.start_time}, end_time: {result.end_time} query: {query}")
        metrics_res = metrics_validator.compare_metrics(result.start_time,
                                                        result.end_time,
                                                        query)

        click.secho(f"Query Name: {query}", fg='bright_white')
        click.secho(f"Error List: {metrics_res.el}", fg='bright_red')
        click.secho(f"Average Error: {metrics_res.me}", fg='bright_yellow')

        click.secho("---------------------------------------------------", fg="cyan")
        report.write("#### Query\n")
        report.write(f"```{query}```\n")
        report.write("#### Average Error\n")
        report.write(f"{metrics_res.me}\n")
        report.write("#### Error List\n")
        report.write(f"{metrics_res.el}\n")
        report.write("\n")
        report.flush()
    report.close()
