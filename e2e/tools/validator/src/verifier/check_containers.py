from typing import List, NamedTuple
from utils.docker_utils import DockerClient


class ServiceReport(NamedTuple):
    service_name: str
    status_success: bool
    log_success: bool
    label_success: bool


class StatusReport(NamedTuple):
    success: bool
    status_success: bool
    log_success: bool
    label_success: bool
    service_reports: List[ServiceReport]


class CheckServiceContainers:


    #TODO: Replace log_errors with class from log_errors.yaml
    def __init__(self, service_names: List[str], log_errors: str) -> None:
        self.service_names = service_names
        self.log_errors = log_errors
        self.dc = DockerClient()
        self.container_data = {}

    
    def load_container_data(self) -> None:
        for service_name in self.service_names:
            relevant_containers = self.dc.get_service_containers_report(service_name)
            self.container_data[service_name] = relevant_containers
    

    def _review_service_logs(self, service_name) -> bool:
        pass 


    def _review_service_label(self, service_name) -> bool:
        pass


    def _review_service_status(self, service_name) -> bool:
        pass


    def review(self) -> StatusReport:
        pass

     
