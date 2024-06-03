from prometheus_api_client import PrometheusConnect
from typing import Tuple, List, NamedTuple
from datetime import datetime
import numpy as np
from validator import config


class MetricsValidatorResult(NamedTuple):
    mae: float  # absolute percentage error list
    mape: float  # absolute percentage error list
    mse: float  # absolute percentage error list
    rmse: float  # absolute percentage error list
    ae: List[float]  # absolute percentage error list
    ape: List[float]  # absolute percentage error list


# TODO: Include Environment Variables if desired
class MetricsValidator:
    # test with float
    def __init__(self, prom: config.Prometheus):
        self.prom_client = PrometheusConnect(prom.url, disable_ssl=True)
        self.step = prom.step

    def custom_metric_query(
        self, start_time: datetime, end_time: datetime, query: str
    ) -> List[dict]:
        try:
            return self.prom_client.custom_query_range(
                query=query, start_time=start_time, end_time=end_time, step=self.step
            )
        except Exception as e:
            print(f"Error querying Prometheus: {e}")
            return []

    def compare_metrics(
        self,
        start_time: datetime,
        end_time: datetime,
        expected_query: str,
        actual_query: str,
    ) -> MetricsValidatorResult:
        expected_metrics = self.custom_metric_query(
            start_time, end_time, expected_query
        )
        actual_metrics = self.custom_metric_query(start_time, end_time, actual_query)

        if not expected_metrics or not actual_metrics:
            raise ValueError("One of the Prometheus queries returned no data")

        print(f"Expected Metrics: {expected_metrics}")
        print(f"Actual Metrics: {actual_metrics}")

        cleaned_expected_metrics = retrieve_timestamp_value_metrics(expected_metrics[0])
        cleaned_actual_metrics = retrieve_timestamp_value_metrics(actual_metrics[0])

        # remove timestamps that do not match
        expected_data, actual_data = acquire_datapoints_with_common_timestamps(
            cleaned_expected_metrics, cleaned_actual_metrics
        )
        return MetricsValidatorResult(
            mae=mean_absolute_error(expected_data, actual_data),
            mape=mean_absolute_percentage_error(expected_data, actual_data),
            mse=mean_squared_error(expected_data, actual_data),
            rmse=root_mean_squared_error(expected_data, actual_data),
            ae=absolute_error(expected_data, actual_data),
            ape=absolute_percentage_error(expected_data, actual_data),
        )


def retrieve_timestamp_value_metrics(
    prom_query_response: dict,
) -> List[Tuple[int, float]]:
    return [
        (int(element[0]), float(element[1]))
        for element in prom_query_response["values"]
    ]


def acquire_datapoints_with_common_timestamps(
    prom_data_list_one, prom_data_list_two: List[Tuple[int, float]]
) -> Tuple[list, list]:
    timestamps_one = {datapoint[0] for datapoint in prom_data_list_one}
    timestamps_two = {datapoint[0] for datapoint in prom_data_list_two}
    common_timestamps = sorted(timestamps_one & timestamps_two)

    list_one_metrics = [
        datapoint[1]
        for datapoint in prom_data_list_one
        if datapoint[0] in common_timestamps
    ]
    list_two_metrics = [
        datapoint[1]
        for datapoint in prom_data_list_two
        if datapoint[0] in common_timestamps
    ]

    return list_one_metrics, list_two_metrics


def absolute_percentage_error(expected_data, actual_data: List[float]) -> List[float]:
    return (
        np.abs(
            (np.array(expected_data) - np.array(actual_data)) / np.array(expected_data)
        )
        * 100
    ).tolist()


def absolute_error(expected_data, actual_data: List[float]) -> List[float]:
    return np.abs(np.array(expected_data) - np.array(actual_data)).tolist()


def mean_absolute_error(expected_data, actual_data: List[float]) -> float:
    return np.mean(np.abs(np.array(expected_data) - np.array(actual_data))).tolist()


def mean_absolute_percentage_error(expected_data, actual_data: List[float]) -> float:
    return np.mean(
        np.abs(
            (np.array(expected_data) - np.array(actual_data)) / np.array(expected_data)
        )
        * 100
    ).tolist()


def mean_squared_error(expected_data, actual_data: List[float]) -> float:
    return np.mean((np.array(expected_data) - np.array(actual_data)) ** 2).tolist()


def root_mean_squared_error(expected_data, actual_data: List[float]) -> float:
    return np.sqrt(mean_squared_error(expected_data, actual_data))


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
