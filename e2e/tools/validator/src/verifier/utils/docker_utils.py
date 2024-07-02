import docker
from docker.models.containers import Container
from docker.models.networks import Network
from docker.errors import APIError
import exceptions
from typing import List, NamedTuple, Generator


class ContainerID(NamedTuple):
    """
    ContainerID is a tuple of (container_id, container_name). 
    It represents the unique identifer of a docker container.
    """
    container_id: str
    container_name: str

    def __str__(self) -> str:
        return f"Container ID: {self.container_id}, Container Name: {self.container_name}"


class NetworkID(NamedTuple):
    """
    NetworkID is a tuple of (network_id, network_name). 
    It represents the unique identifer of a docker network.
    """
    network_id: str
    network_name: str

    def __str__(self) -> str:
        return f"Network ID: {self.network_id}, Network Name: {self.network_name}"



class ContainerReport(NamedTuple):
    """
    ContainerReport is a tuple of (logs, status, labels). 
    It represents the verifiable data of a docker container 
    in a docker compose service. 
    """
    id: ContainerID
    logs: Generator[str, str, str]
    labels: dict
    status: str
     
    def __str__(self) -> str:
        return f"Container Report: \n({str(self.id)})"
    

class NetworkReport(NamedTuple):
    """
    NetworkReport is a tuple of (network_name, connected_container_names). 
    It represents the verifiable data of a docker compose service. 
    """
    id: NetworkID
    connected_container_ids: List[ContainerID]
    def __str__(self) -> str:
        return f"Network Report: \n({str(self.id)})"
    

class DockerClient:
    """
    DockerClient is a Validator API for verifying the validator is functioning by 
    using the Docker SDK. 
    """

    def __init__(self) -> None:
        self.docker_client = docker.from_env()

    
    def _get_service_containers(self, service_name: str, keywords=[]) -> List[Container]:
        try:
            containers = self.docker_client.containers.list()
            service_containers = [container for container in containers
                                  if service_name in container.name]
            cleaned_service_containers = [container for container in service_containers
                                          if any([keyword in container.name 
                                                 for keyword in keywords])]                         
            return cleaned_service_containers
        
        except APIError as e:
            raise exceptions.DockerConnectionException(
                "docker failed to connect to server: " + str(e),
            )
        

    def _get_networks(self, network_name: str, keywords=[]) -> List[Network]:
        try:
            networks = self.docker_client.networks.list(
                names=[network_name]
            )
            cleaned_networks = [network for network in networks
                                if any([keyword in network.name
                                       for keyword in keywords])]
            return cleaned_networks
        
        except APIError as e:
            raise exceptions.DockerConnectionException(
                "docker failed to connect to server: " + str(e),
            )


    def get_service_containers_report(self, service_name: str) -> List[ContainerReport]:
        service_containers = self._get_service_containers(service_name)
        print(service_containers)
        container_reports = []
        for container in service_containers:
            container_reports.append(
                ContainerReport(
                    id=ContainerID(
                        container_id=container.id,
                        container_name=container.name
                    ),
                    status=container.status,
                    labels=container.labels,
                    logs=container.logs(stdout=False, tail=200),
                )
            )
           
        return container_reports
    

    def get_network_report(self, network_name: str) -> List[NetworkReport]:
        networks = self._get_networks(network_name)
        network_reports = []
        for network in networks:
            container_data = [ContainerID(container_id=container.id, 
                                          container_name=container.name) 
                              for container in network.containers]
            network_reports.append(
                NetworkReport(
                    id=NetworkID(
                        network_id=network.id,
                        network_name=network.name
                    ),
                    connected_container_ids=container_data

                )
            )
        return network_reports