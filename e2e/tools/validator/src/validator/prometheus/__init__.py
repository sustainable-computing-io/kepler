from prometheus_api_client import PrometheusConnect
from typing import Tuple, List, NamedTuple
from datetime import datetime
import numpy as np
from datetime import datetime
from validator import config
from statistics import fmean

class MetricsValidatorResult(NamedTuple):
    # error list
    el: List[float]
    # mean error
    me: float


#TODO: Include Environment Variables if desired
class MetricsValidator:
    def __init__(self, prom: config.Prometheus):
        self.prom_client = PrometheusConnect(prom.url, headers=None, disable_ssl=True)
        self.step = prom.step


    def custom_metric_query(self, start_time: datetime, end_time: datetime, query: str):
        return self.prom_client.custom_query_range(
            query=query,
            start_time=start_time,
            end_time=end_time,
            step=self.step
        )


    def compare_metrics(self, start_time: datetime,
                        end_time: datetime,
                        query: str
                        ) -> MetricsValidatorResult:
        query_metrics = self.custom_metric_query(start_time, end_time, query)
        cleaned_expected_metrics = retrieve_timestamp_value_metrics(query_metrics[0])

        return MetricsValidatorResult(
            el=cleaned_expected_metrics,
            me=round(fmean(cleaned_expected_metrics), 3)
        )


def retrieve_timestamp_value_metrics(prom_query_response) -> List[float]:
    acquired_data = []
    for element in prom_query_response['values']:
        acquired_data.append(float(element[1]))
    return acquired_data


# if __name__ == "__main__":
#     prom_metrics_validator = PromMetricsValidator("http://localhost:9091")
#     start_datetime = datetime.strptime("2024-04-10 19:17:53.882176", '%Y-%m-%d %H:%M:%S.%f')
#     end_datetime = datetime.strptime("2024-04-10 19:21:36.320520", '%Y-%m-%d %H:%M:%S.%f')
#     cleaned_validator_data, cleaned_validated_data = prom_metrics_validator.retrieve_energy_metrics_with_queries(
#         start_time=start_datetime,
#         end_time=end_datetime,
#         expected_query="kepler_process_package_joules_total{command='qemu-system-x86'}",
#         actual_query="kepler_node_platform_joules_total{job='vm'}"
#     )
#     # cleaned_validator_data = []
#     # for element in validator_data[0]["values"]:
#     #     cleaned_validator_data.append(float(element[1]))
#     # for element in validator_data[1]["values"]:
#     #     cleaned_validator_data.append(float(element[1]))
#
#     # cleaned_validated_data = []
#     # for element in validated_data[0]["values"]:
#     #     cleaned_validated_data.append(float(element[1]))
#     print(len(cleaned_validator_data))
#     print(len(cleaned_validated_data))
#     print(deltas_func(cleaned_validator_data, cleaned_validated_data))
#     print(percentage_err(cleaned_validator_data, cleaned_validated_data))
#
