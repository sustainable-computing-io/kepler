from typing import NamedTuple, List
from validator import config

# Special Variable Names:
# vm_pid (virtual machine pid), interval (desired range vector)

RAW_PROM_QUERIES = [
    {
        "expected_query": "rate(kepler_process_package_joules_total{{job='metal', pid='{vm_pid}', mode='dynamic'}}[{interval}])",
        "actual_query": "rate(kepler_node_platform_joules_total{{job='vm'}}[{interval}])",
    },
    {
        "expected_query": "rate(kepler_process_platform_joules_total{{job='metal', pid='{vm_pid}', mode='dynamic'}}[{interval}])",
        "actual_query": "rate(kepler_node_platform_joules_total{{job='vm'}}[{interval}])",
    },
    {
        "expected_query": "rate(kepler_process_bpf_cpu_time_ms_total{{job='metal', pid='{vm_pid}'}}[{interval}])",
        "actual_query": "sum by(__name__, job) (rate(kepler_process_bpf_cpu_time_ms_total{{job='vm'}}[{interval}]))",
    },
    {
        "expected_query": "rate(kepler_process_bpf_page_cache_hit_total{{job='metal', pid='{vm_pid}'}}[{interval}])",
        "actual_query": "sum by(__name__, job) (rate(kepler_process_bpf_page_cache_hit_total{{job='vm'}}[{interval}]))",
    },


]

class TestCaseResult(NamedTuple):
    expected_query: str
    actual_query: str


class TestCasesResult(NamedTuple):
    test_cases: List[TestCaseResult]


class TestCases:

    def __init__(self, vm: config.VM, prom: config.Prometheus) -> None:
        self.vm_pid = vm.pid
        self.interval = prom.interval
        self.raw_prom_queries = RAW_PROM_QUERIES
    

    def load_test_cases(self) -> TestCasesResult:
        test_cases = []
        for raw_prom_query in self.raw_prom_queries:
            test_cases.append(TestCaseResult(
                expected_query=raw_prom_query["expected_query"].format(vm_pid=self.vm_pid, interval=self.interval),
                actual_query=raw_prom_query["actual_query"].format(vm_pid=self.vm_pid, interval=self.interval)
            ))
        return TestCasesResult(
            test_cases=test_cases
        )