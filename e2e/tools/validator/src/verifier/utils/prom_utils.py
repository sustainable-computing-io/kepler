from prometheus_api_client import PrometheusConnect
from prometheus_api_client.utils import parse_datetime
from typing import NamedTuple, List

class SeriesData(NamedTuple):
    timestamp: int
    value: float


class PrometheusConnector:

    def __init__(self, url: str, step='3s', headers=None, disable_ssl=True) -> None:
        self.prom = PrometheusConnect(url=url, 
                                      headers=headers, 
                                      disable_ssl=disable_ssl)
        self.start_time = parse_datetime("1h")
        self.end_time = parse_datetime("now")
        self.step = step


    def query_metric(self, query) -> List[SeriesData]:
        results = self.prom.custom_query_range(query=query, 
                                              start_time=self.start_time, 
                                              end_time=self.end_time, 
                                              step=self.step)
        query_results = [result["values"] for result in results]
        return [SeriesData(timestamp=int(values[0]), value=float(values[1])) 
                for values in [result for result in query_results]]

