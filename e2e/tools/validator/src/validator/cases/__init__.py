from typing import NamedTuple, List
from validator import config
import json

def read_json_file(file_path):
    try:
        # Open the file for reading
        with open(file_path, 'r') as file:
            # Load the JSON content into a Python list of dictionaries
            data = json.load(file)
            return data
    except FileNotFoundError:
        print("The file was not found.")
        return []
    except json.JSONDecodeError:
        print("Error decoding JSON. Please check the file format.")
        return []
    except Exception as e:
        print(f"An error occurred: {e}")
        return []

# Special Variable Names:
# vm_pid (virtual machine pid), interval (desired range vector)

# Raw Prometheus Queries, read all the query from the config file

class CaseResult(NamedTuple):
    refined_query: str


class CasesResult(NamedTuple):
    test_cases: List[CaseResult]


class Cases:

    def __init__(self, vm: config.VM, prom: config.Prometheus, query_path: str) -> None:
        self.vm_pid = vm.pid
        self.vm_name = vm.name
        self.interval = prom.interval
        self.raw_prom_queries = read_json_file(query_path)
        self.vm_query = "job='vm'"
        if self.vm_pid != 0:
            self.query = f"pid='{{vm_pid}}'".format(vm_pid=self.vm_pid)
            self.level = "process"
        else:
            self.query = f"vm_id=~'.*{{vm_name}}'".format(vm_name=self.vm_name)
            self.level = "vm"
    
    def load_test_cases(self) -> CasesResult:
        test_cases = []
        for raw_prom_query in self.raw_prom_queries:
            test_cases.append(CaseResult(
                refined_query=raw_prom_query.format(level=self.level, query=self.query, interval=self.interval, vm_query=self.vm_query)
            ))
        return CasesResult(
            test_cases=test_cases
        )