import logging
import re
from typing import NamedTuple

import yaml

from validator import config

logger = logging.getLogger(__name__)


class QueryTemplate:
    def __init__(self, query: str, promql_vars: dict[str, str]) -> None:
        self._original = query
        self._promql_vars = promql_vars
        self._promql = query.format(**promql_vars)

    @property
    def original(self) -> str:
        return self._original

    @property
    def promql(self) -> str:
        return self._promql

    @property
    def one_line(self) -> str:
        return re.sub(r"\n", "", re.sub(r"\s+", " ", self._promql))

    @property
    def metric_name(self) -> str:
        metric = re.search(r"kepler_[a-z_]+_total", self._promql)
        if metric is None:
            return f"unknown_{hash(self._promql)}"

        return metric.group(0)

    @property
    def mode(self) -> str:
        m = re.search(r"mode=['\"]([a-z]+)['\"]", self._promql)
        if not m:
            return "unknown"
        return m.group(1)


class Validation(NamedTuple):
    name: str
    expected: QueryTemplate
    actual: QueryTemplate


def read_validations(file_path: str, promql_vars: dict[str, str]) -> list[Validation]:
    with open(file_path) as file:
        yml = yaml.safe_load(file)
        return [
            Validation(
                name=v["name"],
                expected=QueryTemplate(v["expected"], promql_vars),
                actual=QueryTemplate(v["actual"], promql_vars),
            )
            for v in yml["validations"]
        ]


class Loader:
    def __init__(self, cfg: config.Validator):
        self.cfg = cfg

    def load(self) -> list[Validation]:
        promql_vars = {}

        vm = self.cfg.metal.vm
        if vm.pid != 0:
            promql_vars["level"] = "process"
            promql_vars["vm_selector"] = f'pid="{vm.pid}"'
        else:
            promql_vars["level"] = "vm"
            promql_vars["vm_selector"] = f'vm_id=~".*{vm.name}"'

        prom = self.cfg.prometheus
        promql_vars["rate_interval"] = prom.rate_interval
        promql_vars["metal_job_name"] = prom.job.metal
        promql_vars["vm_job_name"] = prom.job.vm

        logger.debug("promql_vars: %s", promql_vars)

        return read_validations(self.cfg.validations_file, promql_vars)
