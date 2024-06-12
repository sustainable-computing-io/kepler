from typing import NamedTuple, List
from validator import config
import json
import re

def read_json_file(file_path):
    try:
        # Open the file for reading
        with open(file_path, 'r') as file:
            # Load the JSON content into a Python list of dictionaries
            data = json.load(file)
            return data
    except FileNotFoundError:
        print("The file was not found.")
        return None
    except json.JSONDecodeError:
        print("Error decoding JSON. Please check the file format.")
        return None
    except Exception as e:
        print(f"An error occurred: {e}")
        return None

def create_file_name(query):
    metric_name = re.search(r'kepler_[a-z_]+_joules_total', query).group(0)
    mode = re.search(r"mode='([a-z]+)'", query).group(1)
    return f"{metric_name}+{mode}"

# Special Variable Names:
# vm_pid (virtual machine pid), interval (desired range vector)

# Raw Prometheus Queries, read all the query from the config file
class RawCaseResult(NamedTuple):
    file_name : str
    query : str

# Refined query
class CaseResult(NamedTuple):
    refined_query: str

class CasesResult(NamedTuple):
    mse_test_cases: List[CaseResult]
    mape_test_cases: List[CaseResult]
    raw_query_results: List[RawCaseResult]


class Cases:

    def __init__(self, metal_job_name: str, vm_job_name: str, vm: config.VM, prom: config.Prometheus, query_path: str) -> None:
        self.vm_pid = vm.pid
        self.vm_name = vm.name
        self.metal_job_name = metal_job_name
        self.vm_job_name = vm_job_name
        self.interval = prom.interval
        self.queries = read_json_file(query_path)
        self.raw_prom_queries = self.queries["raw"]
        self.mse_prom_queries = self.queries["mse"]
        self.mape_prom_queries = self.queries["mape"]
        # TODO self.mape_prom_queries = queries["mape"]
        self.vm_query = f"job='{self.vm_job_name}'"
        if self.vm_pid != 0:
            self.query = f"pid='{{vm_pid}}'".format(vm_pid=self.vm_pid)
            self.level = "process"
        else:
            self.query = f"vm_id=~'.*{{vm_name}}'".format(vm_name=self.vm_name)
            self.level = "vm"

    def load_test_cases(self) -> CasesResult:
        mse_test_cases = []
        for mse_prom_query in self.mse_prom_queries:
            mse_test_cases.append(CaseResult(
                refined_query=mse_prom_query.format(
                    metal_job_name = self.metal_job_name, vm_job_name = self.vm_job_name,
                    level=self.level, query=self.query, interval=self.interval, vm_query=self.vm_query
                )
            ))
        mape_test_cases = []
        for mape_prom_query in self.mape_prom_queries:
            mape_test_cases.append(CaseResult(
                refined_query=mape_prom_query.format(
                    metal_job_name = self.metal_job_name, vm_job_name = self.vm_job_name,
                    level=self.level, query=self.query, interval=self.interval, vm_query=self.vm_query
                )
            ))

        raw_test_cases = []
        for raw_prom_query in self.raw_prom_queries:
            raw_query = raw_prom_query.format(
                metal_job_name = self.metal_job_name, vm_job_name = self.vm_job_name,
                level=self.level, query=self.query, interval=self.interval, vm_query=self.vm_query
            )
            file_name = create_file_name(raw_query)
            raw_test_cases.append(RawCaseResult(file_name=file_name, query=raw_query))
        return CasesResult(mse_test_cases=mse_test_cases, mape_test_cases=mape_test_cases, raw_query_results=raw_test_cases)
