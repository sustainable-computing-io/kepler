from prometheus_api_client import PrometheusConnect
from typing import Tuple, List
from datetime import datetime
import numpy as np
import statistics
import requests
from datetime import datetime, timedelta
import requests

#TODO: Include Environment Variables if desired
# class PromMetricsValidator:
#     def __init__(self, endpoint: str, headers=None, disable_ssl=True) -> None:
#         self.prom_client = PrometheusConnect(endpoint, headers=None, disable_ssl=disable_ssl)
        

def merge_prom_metric_list(prom_query_result: list) -> List[Tuple[str, float]]:
    cleaned_data = []
    # same metrics will have same timestamps
    for index, query in enumerate(prom_query_result):
        for element in query["values"]:
            if index == 0:
                cleaned_data.append( [element[0], float(element[1])] )
            else:
                cleaned_data[index][1] += float(element[1])
    return cleaned_data
    

def disjunct_on_timestamps(prom_data_list_one, prom_data_list_two) -> Tuple[list, list]:
    common_timestamps = [int(datapoint[0]) for datapoint in prom_data_list_one if int(datapoint[0]) in [int(datapoint[0]) for datapoint in prom_data_list_two]]
    # necessary to sort timestamps?
    common_timestamps.sort()
    list_one_metrics = []
    list_two_metrics = []
    for timestamp in common_timestamps:
        for list_one_datapoint in prom_data_list_one:
            if int(list_one_datapoint[0]) == timestamp:
                list_one_metrics.append(list_one_datapoint[1])
        for list_two_datapoint in prom_data_list_two:
            if int(list_two_datapoint[0]) == timestamp:
                list_two_metrics.append(list_two_datapoint[1])
    return list_one_metrics, list_two_metrics


def compare_metrics(endpoint: str, disable_ssl, start_time: datetime, end_time: datetime, expected_query: str, expected_query_labels: dict, actual_query: str, actual_query_labels: dict) -> Tuple[List[float], List[float]]:   
    prom_client = PrometheusConnect(endpoint, headers=None, disable_ssl=disable_ssl)

    # Define your query and time range
    expected_query_test = "kepler_process_package_joules_total{job='metal',pid='99498'}"
    actual_query_test = "kepler_node_platform_joules_total{job='vm'}"
    

    # Prometheus query API endpoint
    api_endpoint = "http://localhost:9091/api/v1/query_range"

    # Query parameters
    params_expected = {
        'query': expected_query_test,
        'start': int(start_time.timestamp()),
        'end': int(end_time.timestamp()),
        'step': '3s'  # Sample step size, adjust as needed
    }
    params_actual = {
        'query': actual_query_test,
        'start': int(start_time.timestamp()),
        'end': int(end_time.timestamp()),
        'step': '3s'  # Sample step size, adjust as needed
    }
    

    # Make the GET request
    response_expected = requests.get(api_endpoint, params=params_expected)
    response_actual = requests.get(api_endpoint, params=params_actual)

    # Check if the request was successful (status code 200)
    if response_expected.status_code == 200 and response_actual.status_code == 200:
        data_expected = response_expected.json()
        data_actual = response_actual.json()
        print(data_expected)  
        print(data_actual)
    else:
        print(f"Error: {response_expected.status_code} - {response_expected.text}")

    expected_metrics = prom_client.get_metric_range_data(
        metric_name=expected_query,
        label_config=expected_query_labels.copy(),
        start_time=start_time,
        end_time=end_time,
        params={
            "step": "3s"
        }

    )
    actual_metrics = prom_client.get_metric_range_data(
        #metric_name=actual_query,
        metric_name="kepler_node_platform_joules_total{job='vm'}",
        #label_config=actual_query_labels.copy(),
        start_time=start_time,
        end_time=end_time,
        params={
            "step": "3s"
        }
    )
    print(expected_metrics)
    print(actual_metrics)
    # clean data to acquire only lists
    expected_data = merge_prom_metric_list(expected_metrics)
    actual_data = merge_prom_metric_list(actual_metrics)
    
    # remove timestamps that do not match
    return disjunct_on_timestamps(expected_data, actual_data)


def absolute_percentage_error(expected_data, actual_data) -> List[float]:
    expected_data = np.array(expected_data)
    actual_data = np.array(actual_data)

    absolute_percentage_error = np.abs((expected_data - actual_data) / expected_data) * 100
    return absolute_percentage_error.tolist()


def absolute_error(expected_data, actual_data) -> List[float]:
    expected_data = np.array(expected_data)
    actual_data = np.array(actual_data)

    absolute_error = np.abs(expected_data - actual_data)
    return absolute_error.tolist()


def mean_absolute_error(expected_data, actual_data) -> float:
    return statistics.mean(absolute_error(expected_data, actual_data))


def mean_absolute_percentage_error(expected_data, actual_data) -> float:
    return statistics.mean(absolute_percentage_error(expected_data, actual_data))



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
    
