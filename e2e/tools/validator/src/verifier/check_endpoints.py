from typing import List


class CheckVMEndpoints:

    #TODO: Replace prom_query_file with class from verifier.yaml
    def __init__(self, prom_url, prom_query_file: str) -> None:
        self.prom_url = prom_url
        self.prom_query_file = prom_query_file

    