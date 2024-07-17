import logging
import re
from datetime import datetime
from typing import List, NamedTuple, Protocol, Tuple

import numpy as np
import numpy.typing as npt
from prometheus_api_client import PrometheusConnect

from validator.config import Prometheus as PromConfig

logger = logging.getLogger(__name__)


class Sample(NamedTuple):
    """
    Sample is a tuple of (timestamp, value) and represents a single sample
    of a prometheus time series.
    """

    timestamp: int
    value: float

    @property
    def datetime(self) -> datetime:
        return datetime.fromtimestamp(self.timestamp)

    def __str__(self) -> str:
        return f"({self.timestamp}: {self.value})"


class Series:
    """
    Series is a list of Samples. It also holds the query used
    to generate the samples
    """

    query: str

    def __init__(self, query: str, samples: list[tuple[int, str]]):
        self.query = query
        self.samples = [Sample(int(s[0]), float(s[1])) for s in samples]

    @classmethod
    def from_samples(cls, query: str, samples: list[Sample]) -> "Series":
        s = Series(query, [])
        s.samples = samples[:]
        return s

    @property
    def timestamps(self) -> list[float]:
        return [s.timestamp for s in self.samples]

    @property
    def values(self) -> list[float]:
        return [s.value for s in self.samples]

    def __str__(self) -> str:
        return f"{self.query}\n: {[ str(s) for s in self.samples]}"


class ValueOrError(NamedTuple):
    """
    ValueOrError is a tuple of (value, error) and represents
    either a value or an error
    """

    value: float
    error: str | None = None

    def __str__(self) -> str:
        if self.error is None:
            return str(self.value)

        return f"Error: {self.error}"


class Result(NamedTuple):
    expected_series: Series
    actual_series: Series
    mse: ValueOrError
    mape: ValueOrError

    def print(self):
        print("Expected:")
        print("────────────────────────────────────────")
        print(f" {self.expected_series.query}")
        print(f" {self.expected_series.values}")
        print("\t\t\t\t⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯\n")

        print("Actual:")
        print("────────────────────────────────────────\n")
        print(f"{self.actual_series.query}")
        print(f"{self.actual_series.values}")
        print("\t\t\t\t⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯\n")

        print(f"MSE : {self.mse}")
        print(f"MAPE: {self.mape}")
        print("\t\t\t\t━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")


def validate_arrays(actual: npt.ArrayLike, expected: npt.ArrayLike) -> tuple[npt.ArrayLike, npt.ArrayLike]:
    actual, expected = np.array(actual), np.array(expected)
    if len(actual) != len(expected):
        raise ValueError(f"actual and expected must be of equal length: {len(actual)} != {len(expected)}")
    elif len(actual) == 0 or len(expected) == 0:
        raise ValueError(f"actual ({len(actual)}) and expected ({len(expected)}) must not be empty")

    return (actual, expected)


def mse(actual: npt.ArrayLike, expected: npt.ArrayLike) -> ValueOrError:
    try:
        actual, expected = validate_arrays(actual, expected)
        return ValueOrError(value=np.square(np.subtract(actual, expected)).mean())
    except Exception as e:
        return ValueOrError(value=0, error=str(e))


def mape(actual: npt.ArrayLike, expected: npt.ArrayLike) -> ValueOrError:
    try:
        actual, expected = validate_arrays(actual, expected)
        return ValueOrError(value=100 * np.abs(np.divide(np.subtract(actual, expected), actual)).mean())
    except Exception as e:
        return ValueOrError(value=0, error=str(e))


def filter_by_equal_timestamps(a: Series, b: Series) -> tuple[Series, Series]:
    """
    filter_by_equal_timestamps will filter out samples from a and b
    that have the same timestamp.
    E.g. given
        a: (t1, a1), (t2, a2), (t3, a3)
        b: (t1, b1), (t3, b3)
    it returns
        a: (t1, a1), (t3, a3)
        b: (t1, b1), (t3, b3)
    """

    filtered_a = []
    filterd_b = []

    idx_a, idx_b = 0, 0

    while idx_a < len(a.samples) and idx_b < len(b.samples):
        if a.samples[idx_a].timestamp == b.samples[idx_b].timestamp:
            filtered_a.append(a.samples[idx_a])
            filterd_b.append(b.samples[idx_b])
            idx_a += 1
            idx_b += 1
        elif a.samples[idx_a].timestamp < b.samples[idx_b].timestamp:
            idx_a += 1
        else:
            idx_b += 1

    return Series.from_samples(a.query, filtered_a), Series.from_samples(b.query, filterd_b)


class SeriesError(Exception):
    def __init__(self, query: str, expected: int, got: int):
        self.query = strip_query(query)
        self.expected = expected
        self.got = got

    def __str__(self) -> str:
        return f"Query: {self.query}\n\tExpected: {self.expected}\n\tGot: {self.got}"


def strip_query(query: str) -> str:
    one_line = re.sub(r"\n", " ", query)
    one_line = re.sub(r"\s+", " ", one_line)
    return one_line


class Queryable(Protocol):
    # ruff: noqa: ARG002 (we don't care about the arguments here)
    def range_query(self, query: str, start: datetime, end: datetime) -> list[Series]:
        return []


def to_metric(labels: dict[str, str]) -> str:
    name = labels["__name__"]
    rest = ", ".join([f'{k}="{v}"' for k, v in labels.items() if k != "__name__"])
    return f"{name}{{{rest}}}"


class PrometheusClient:
    def __init__(self, cfg: PromConfig):
        self.prom = PrometheusConnect(cfg.url, disable_ssl=True)
        self.step = cfg.step

    def range_query(self, query: str, start: datetime, end: datetime) -> list[Series]:
        """
        range_query_single_series returns a single series from the query
        """
        logger.info(f"running query {strip_query(query)} with step {self.step}")
        series = self.prom.custom_query_range(query=query, start_time=start, end_time=end, step=self.step)

        return [Series(query, s["values"]) for s in series]

    def kepler_build_info(self) -> list[str]:
        resp = self.prom.custom_query(query="kepler_exporter_build_info")
        build_info = [r["metric"] for r in resp]
        return [to_metric(b) for b in build_info]


class Comparator:
    def __init__(self, client: Queryable):
        self.prom_client = client

    def single_series(self, query: str, start: datetime, end: datetime) -> Series:
        series = self.prom_client.range_query(query, start, end)

        if len(series) != 1:
            raise SeriesError(query, 1, len(series))

        return series[0]

    def compare(
        self,
        start: datetime,
        end: datetime,
        expected_query: str,
        actual_query: str,
    ) -> Result:
        expected_series = self.single_series(expected_query, start, end)
        actual_series = self.single_series(actual_query, start, end)

        expected, actual = filter_by_equal_timestamps(expected_series, actual_series)

        return Result(
            mse=mse(actual.values, expected.values),
            mape=mape(actual.values, expected.values),
            expected_series=expected_series,
            actual_series=actual_series,
        )
