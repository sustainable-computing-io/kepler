import pytest
from validator.prometheus import (absolute_percentage_error, 
                                  absolute_error, 
                                  mean_absolute_error, 
                                  mean_absolute_percentage_error,
                                  mean_squared_error,
                                  root_mean_squared_error,
                                  retrieve_timestamp_value_metrics,
                                  acquire_datapoints_with_common_timestamps
                                  )


def test_retrieve_timestamp_value_metrics():
    prom_response_sample = [{'metric': 
                             {'command': 'worker', 'container_id': 'emulator', 'instance': 'kepler:9100', 
                              'job': 'metal', 'mode': 'dynamic', 'pid': '17341', 'source': 'intel_rapl', 
                              'vm_id': 'machine-qemu-1-ubuntu22.04'}, 'values': 
                              [[1716252592, '0.09833333333333548'], [1716252595, '0.08933333333333356'], 
                               [1716252598, '0.12299999999999991'], [1716252601, '0.10299999999999916'], 
                               [1716252604, '0.10566666666666592'], [1716252607, '1.3239999999999996'], 
                               [1716252610, '3.7116666666666664'], [1716252613, '5.317999999999999']]}]
    res = retrieve_timestamp_value_metrics(prom_response_sample[0])
    assert len(prom_response_sample[0]["values"]) == len(res)
    sample_datapoint =  prom_response_sample[0]["values"][0]
    sample_datapoint[1] = float(sample_datapoint[1])
    assert res[0] == sample_datapoint


def test_acquire_datapoints_with_common_timestamps():
    prom_one_sample_empty = [[1716252592, 0.09833333333333548], [1716252595, 0.08933333333333356], 
                        [1716252598, 0.12299999999999991], [1716252601, 0.10299999999999916]]
    prom_two_sample_empty = [[1716252591, 0.09833333333333548], [1716252596, 0.08933333333333356], 
                        [1716252599, 0.12299999999999991], [1716252600, 0.10299999999999916]]
    
    one_sample_empty, two_sample_empty = acquire_datapoints_with_common_timestamps(prom_one_sample_empty, 
                                                                                   prom_two_sample_empty)
    assert len(one_sample_empty) == 0 and len(two_sample_empty) == 0

    prom_one_sample_three_common_timestamps = [[1716252592, 0.09833333333333548], 
                                               [1716252595, 0.08933333333333356], 
                                               [1716252598, 0.12299999999999991], 
                                               [1716252601, 0.10299999999999916]]
    prom_two_sample_three_common_timestamps =  [[1716252592, 0.09833333333333548], 
                                                [1716252595, 0.08933333333333356], 
                                                [1716252598, 0.12299999999999999], 
                                                [1716252602, 0.10299999999999916]]

    one_sample_three, two_sample_three = acquire_datapoints_with_common_timestamps(prom_one_sample_three_common_timestamps, 
                                                                                   prom_two_sample_three_common_timestamps)

    assert len(one_sample_three) == 3 and len(two_sample_three) == 3
    assert one_sample_three[-1] == 0.12299999999999991
    assert two_sample_three[-1] == 0.12299999999999999


def test_absolute_percentage_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = absolute_percentage_error(expected_data, actual_data)
    assert result == [0.0, 0.0, 0.0]
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = absolute_percentage_error(expected_data, actual_data)
    rounded_results = list(map(lambda x: round(x, 2), result))
    assert rounded_results == [10.0, 10.0, 3.33]


def test_absolute_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = absolute_error(expected_data, actual_data)
    assert result == [0.0, 0.0, 0.0]
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = absolute_error(expected_data, actual_data)
    rounded_results = list(map(lambda x: round(x, 2), result))

    assert rounded_results == [0.1, 0.2, 0.1]


def test_mean_absolute_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = mean_absolute_error(expected_data, actual_data)
    assert result == 0.0
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = mean_absolute_error(expected_data, actual_data)
    assert round(result, 3) == 0.133


def test_mean_absolute_percentage_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = mean_absolute_percentage_error(expected_data, actual_data)
    assert result == 0.0
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = mean_absolute_percentage_error(expected_data, actual_data)
    assert round(result, 3) == 7.778


def test_mean_squared_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = mean_squared_error(expected_data, actual_data)
    assert result == 0.0
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = mean_squared_error(expected_data, actual_data)
    assert round(result, 2) == 0.02


def test_root_mean_squared_error():
    # equal lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.0, 2.0, 3.0]
    result = root_mean_squared_error(expected_data, actual_data)
    assert result == 0.0
    # different lists
    expected_data = [1.0, 2.0, 3.0]
    actual_data = [1.1, 2.2, 2.9]
    result = root_mean_squared_error(expected_data, actual_data)
    assert round(result, 3) == 0.141



