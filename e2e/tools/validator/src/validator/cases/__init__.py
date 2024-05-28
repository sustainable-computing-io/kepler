from typing import NamedTuple, List
from validator import config

# Special Variable Names:
# vm_pid (virtual machine pid), interval (desired range vector)

RAW_PROM_QUERIES = [
    {
        "expected_query": "rate(kepler_{level}_package_joules_total{{{query}, mode='dynamic'}}[{interval}])",
        "actual_query": "rate(kepler_node_platform_joules_total[{interval}])",
    },
    {
        "expected_query": "rate(kepler_{level}_platform_joules_total{{{query}, mode='dynamic'}}[{interval}])",
        "actual_query": "rate(kepler_node_platform_joules_total[{interval}])",
    },
    {
        "expected_query": "rate(kepler_{level}_bpf_cpu_time_ms_total{{{query}}}[{interval}])",
        "actual_query": "sum by(__name__, job) (rate(kepler_process_bpf_cpu_time_ms_total[{interval}]))",
    },
    {
        "expected_query": "rate(kepler_{level}_bpf_page_cache_hit_total{{{query}}}[{interval}])",
        "actual_query": "sum by(__name__, job) (rate(kepler_process_bpf_page_cache_hit_total[{interval}]))",
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
        self.vm_name = vm.name
        self.interval = prom.interval
        self.raw_prom_queries = RAW_PROM_QUERIES

        if self.vm_pid != 0:
            self.query = f"pid='{{vm_pid}}'".format(vm_pid=self.vm_pid)
            self.level = "process"
        else:
            self.query = f"vm_id=~'.*{{vm_name}}'".format(vm_name=self.vm_name)
            self.level = "vm"
    
    def load_test_cases(self) -> TestCasesResult:
        test_cases = []
        for raw_prom_query in self.raw_prom_queries:
            test_cases.append(TestCaseResult(
                expected_query=raw_prom_query["expected_query"].format(level=self.level, query=self.query, interval=self.interval),
                actual_query=raw_prom_query["actual_query"].format(interval=self.interval)
            ))
        return TestCasesResult(
            test_cases=test_cases
        )