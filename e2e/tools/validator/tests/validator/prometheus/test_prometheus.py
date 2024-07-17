import datetime

import numpy as np
import pytest

from validator.config import (
    Prometheus as PromConfig,
)
from validator.config import (
    PrometheusJob as Job,
)
from validator.prometheus import (
    Comparator,
    Series,
    filter_by_equal_timestamps,
    mape,
    mse,
)


@pytest.fixture
def prom_response():
    return [
        {
            "metric": {
                "command": "worker",
                "container_id": "emulator",
                "instance": "kepler:9100",
                "job": "metal",
                "mode": "dynamic",
                "pid": "17341",
                "source": "intel_rapl",
                "vm_id": "machine-qemu-1-ubuntu22.04",
            },
            "values": [
                [1716252592, "0.09833333333333548"],
                [1716252595, "0.08933333333333356"],
                [1716252598, "0.12299999999999991"],
                [1716252601, "0.10299999999999916"],
                [1716252604, "0.10566666666666592"],
                [1716252607, "1.3239999999999996"],
                [1716252610, "3.7116666666666664"],
                [1716252613, "5.317999999999999"],
            ],
        }
    ]


def test_series(prom_response):
    promql = """kepler_process_cpu_bpf_time_total{job="vm", mode="dynamic"}"""
    resp_values = prom_response[0]["values"]
    s = Series(promql, resp_values)

    assert s.query == promql
    assert len(s.samples) == len(resp_values)

    timestamps = [v[0] for v in resp_values]
    assert s.timestamps == timestamps
    # NOTE: values are stored in float whereas prometheus response has values in string
    values = [v[1] for v in resp_values]
    assert [str(v) for v in s.values] == values


def test_filter_by_equal_timestamps():
    # fmt: off
    def v_a(x): return x * 1.1
    def s_a(x): return (x, str(v_a(x)))

    def v_b(x): return x * 1.1 + 1
    def s_b(x): return (x, str(v_b(x)))


    inputs = [
        {
            "a": [ s_a(1), s_a(2), s_a(3), s_a(4), ],
            "b": [ s_b(1), s_b(2), s_b(3), s_b(4), ],
            "expected_a": [ v_a(1), v_a(2), v_a(3), v_a(4), ],
            "expected_b": [ v_b(1), v_b(2), v_b(3), v_b(4), ],
        },

        {
            "a": [ s_a(1), s_a(2), s_a(3), s_a(4) ],
            "b": [         s_b(2), s_b(3), s_b(4) ],
            "expected_a": [ v_a(2), v_a(3), v_a(4) ],
            "expected_b": [ v_b(2), v_b(3), v_b(4) ],
        },
        {
            "a": [ s_a(1), s_a(2), s_a(3)         ],
            "b": [ s_b(1), s_b(2), s_b(3), s_b(4) ],
            "expected_a": [ v_a(1), v_a(2), v_a(3)],
            "expected_b": [ v_b(1), v_b(2), v_b(3)],
        },
        {
            "a": [ s_a(1),         s_a(3), s_a(4), ],
            "b": [ s_b(1), s_b(2), s_b(3), s_b(4), ],
            "expected_a": [ v_a(1), v_a(3), v_a(4), ],
            "expected_b": [ v_b(1), v_b(3), v_b(4), ],
        },
        {
            "a": [ s_a(1),                 s_a(4), ],
            "b": [ s_b(1), s_b(2), s_b(3), s_b(4), ],
            "expected_a": [ v_a(1), v_a(4), ],
            "expected_b": [ v_b(1), v_b(4), ],
        },
        {
            "a": [ s_a(1), s_a(2), s_a(3), s_a(4) ],
            "b": [ s_b(100), s_b(200), s_b(300), s_b(400), ],
            "expected_a": [],
            "expected_b": [],
        },
    ]
    # fmt: on

    for s in inputs:
        a = Series("a", s["a"])
        b = Series("b", s["b"])
        exp_a = s["expected_a"]
        exp_b = s["expected_b"]

        got_a, got_b = filter_by_equal_timestamps(a, b)

        assert len(got_a.samples) == len(exp_a)
        assert got_a.values == exp_a

        assert len(got_b.samples) == len(exp_b)
        assert got_b.values == exp_b

        # swap and check if the result still holds
        a, b = b, a
        exp_a, exp_b = exp_b, exp_a

        got_a, got_b = filter_by_equal_timestamps(a, b)

        assert len(got_a.samples) == len(exp_a)
        assert got_a.values == exp_a

        assert len(got_b.samples) == len(exp_b)
        assert got_b.values == exp_b


def test_mse():
    # fmt: off
    inputs = [{
        "a": [ 1.0, 2.0, 3.0, 4.0, ],
        "b": [ 1.0, 2.0, 3.0, 4.0, ],
        "mse": 0.0,
        "mape": 0.0,
    }, {
        "a": [ -1.0, -2.0, -3.0, -4.0, ],
        "b": [ -1.0, -2.0, -3.0, -4.0, ],
        "mse": 0.0,
        "mape": 0.0,
    }, {
        "a": [ 1.0, -2.0, 3.0, 4.0, ],
        "b": [ 1.0, -2.0, 3.0, 4.0, ],
        "mse": 0.0,
        "mape": 0.0,
    }, {
        "a": [ 1, 2, 3, 4, ],
        "b": [ 1.0, 2.0, 3.0, 4.0, ],
        "mse": 0.0,
        "mape": 0.0,
    }, {
        "a": [ 1, 2, 3, ],
        "b": [ 4, 5, 6, ],
        "mse": 9.0, # (1 - 4)^2 + (2 - 5)^2 + (3 - 6)^2 / 3
        "mape": 183.3333,
    }, {
        "a": [ 1.5, 2.5, 3.5 ],
        "b": [ 1.0, 2.0, 3.0 ],
        "mse": 0.25, # 3 x (0.5^2) / 3
        "mape": 22.5396,
    }, {
        "a": [ 1, -2, 3 ],
        "b": [ -1, 2, -3 ],
        "mse": 18.6666, # 2.0^2 + 4.0^2 + 6.0^2 / 3
        "mape": 200.0,
    }]
    # fmt: on

    for s in inputs:
        a = s["a"]
        b = s["b"]

        expected_mse = s["mse"]
        assert expected_mse == pytest.approx(mse(a, b).value, rel=1e-3)

        expected_mape = s["mape"]
        assert expected_mape == pytest.approx(mape(a, b).value, rel=1e-3)


def test_mse_with_large_arrays():
    actual = np.random.rand(1000)
    expected = np.random.rand(1000)
    assert mse(actual, expected).value >= 0.0  # MSE should always be non-negative


def test_mse_expections():
    v = mse([], [])
    assert v.value == 0.0
    assert v.error is not None
    assert str(v) == "Error: actual (0) and expected (0) must not be empty"


def test_mse_with_different_lengths():
    actual = [1, 2, 3]
    expected = [1, 2]
    v = mse(actual, expected)
    assert v.value == 0.0
    assert v.error is not None
    assert str(v) == "Error: actual and expected must be of equal length: 3 != 2"


class MockPromClient:
    def __init__(self, responses):
        self.responses = responses

    def range_query(self, query: str, _start: datetime.datetime, _end: datetime.datetime) -> list[Series]:
        return [Series(query, r["values"]) for r in self.responses]


@pytest.fixture
def prom_config():
    return PromConfig(
        url="",
        rate_interval="12s",
        job=Job(metal="metal", vm="vm"),
        step="3s",
    )


def test_comparator_single_series(prom_response):
    c = Comparator(MockPromClient(prom_response))

    promql = """kepler_process_cpu_bpf_time_total{job="vm", mode="dynamic"}"""
    # ruff: noqa: DTZ001 (Suppressed as time-zone aware object creation is not necessary for this use case)
    series = c.single_series(
        promql,
        datetime.datetime(2022, 1, 1),
        datetime.datetime(2022, 1, 2),
    )

    assert series is not None
    assert series.query == promql

    values = [float(s[1]) for s in prom_response[0]["values"]]
    assert series.values == values
