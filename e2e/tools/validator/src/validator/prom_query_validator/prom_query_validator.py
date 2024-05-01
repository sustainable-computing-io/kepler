from prometheus_api_client import PrometheusConnect
from typing import Tuple, List
from datetime import datetime

#TODO: Include Environment Variables if desired
class PromMetricsValidator:
    def __init__(self, endpoint: str, headers=None, disable_ssl=True) -> None:
        self.prom_client = PrometheusConnect(endpoint, headers=None, disable_ssl=disable_ssl)
        


    def compare_metrics(self, start_time: datetime, end_time: datetime, expected_query: str, expected_query_labels: dict, actual_query: str, actual_query_labels: dict) -> Tuple[List[float], List[float]]:
        # parsed_start_time = parse_datetime(start_time)
        # if parsed_start_time is None:
        #     raise ValueError("Invalid start time")
        #
        # parsed_end_time = parse_datetime(end_time)
        # if parsed_end_time is None:
        #     raise ValueError("Invalid end time")


        expected_result = self.prom_client.get_metric_range_data(
            metric_name=expected_query,
            label_config=expected_query_labels.copy(),
            start_time=start_time,
            end_time=end_time,
        )

        actual_metrics = self.prom_client.get_metric_range_data(
            metric_name=actual_query,
            label_config=actual_query_labels.copy(),
            start_time=start_time,
            end_time=end_time,
        )
        # clean data to acquire only lists
        cleaned_validator_data = []
        for query in expected_result:
            for index, element in enumerate(query["values"]):
                if len(cleaned_validator_data) < index + 1:
                    cleaned_validator_data.append(float(element[1]))
                else:
                    cleaned_validator_data[index] += float(element[1])

        cleaned_validated_data = []
        for query in actual_metrics:
            for index, element in enumerate(query["values"]):
                if len(cleaned_validated_data) < index + 1:
                    cleaned_validated_data.append(float(element[1]))
                else:

                    cleaned_validated_data[index] += float(element[1])
        return cleaned_validator_data, cleaned_validated_data


def deltas_func(validator_data, validated_data) -> List[float]:
    delta_list = []
    for validator_element, validated_element in zip(validator_data, validated_data):
        delta_list.append(abs(validator_element - validated_element))
    return delta_list

def percentage_err(validator_data, validated_data) -> List[float]:
    percentage_err_list = []
    for validator_element, validated_element in zip(validator_data, validated_data):
        percentage_err_list.append(abs((validator_element - validated_element) / validator_element) * 100)
    return percentage_err_list


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
    
