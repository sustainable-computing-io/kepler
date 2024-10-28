import logging
import re
from datetime import datetime
from typing import NamedTuple, Protocol

import numpy.typing as npt
from prometheus_api_client import PrometheusConnect
from sklearn.metrics import mean_absolute_error, mean_absolute_percentage_error, mean_squared_error

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
        # ruff: noqa: DTZ006 (Suppressed time-zone aware object creation as it is not required)
        return datetime.fromtimestamp(self.timestamp)

    def __str__(self) -> str:
        return f"({self.timestamp}: {self.value})"


class Series:
    """
    Series is a list of Samples. It also holds the query used
    to generate the samples
    """

    query: str

    def __init__(self, query: str, samples: list[tuple[int, str]], labels: dict[str, str]):
        self.query = query
        self.samples = [Sample(int(s[0]), float(s[1])) for s in samples]
        self.labels = labels

    @classmethod
    def from_samples(cls, query: str, samples: list[Sample], labels: dict[str, str]) -> "Series":
        s = Series(query, [], {})
        s.samples = samples[:]
        s.labels = labels
        return s

    @property
    def timestamps(self) -> list[float]:
        return [s.timestamp for s in self.samples]

    @property
    def values(self) -> list[float]:
        return [s.value for s in self.samples]

    def __str__(self) -> str:
        return f"{self.query}\n: {[ str(s) for s in self.samples]}, {self.labels}"


class ValueOrError(NamedTuple):
    """
    ValueOrError is a tuple of (value, error) and represents
    either a value or an error
    """

    value: float
    error: str | None = None

    def __str__(self) -> str:
        if self.error is None:
            return f"{self.value:.2f}"

        return f"Error: {self.error}"


class Result(NamedTuple):
    actual_series: Series
    predicted_series: Series

    actual_dropped: int
    predicted_dropped: int

    mse: ValueOrError
    mape: ValueOrError
    mae: ValueOrError


def mse(actual: npt.ArrayLike, predicted: npt.ArrayLike) -> ValueOrError:
    try:
        return ValueOrError(value=mean_squared_error(actual, predicted))

    # ruff: noqa: BLE001 (Suppressed as we want to catch all exceptions here)
    except Exception as e:
        return ValueOrError(value=0, error=str(e))


def mape(actual: npt.ArrayLike, predicted: npt.ArrayLike) -> ValueOrError:
    try:
        return ValueOrError(value=mean_absolute_percentage_error(actual, predicted) * 100)

    # ruff: noqa: BLE001 (Suppressed as we want to catch all exceptions here)
    except Exception as e:
        return ValueOrError(value=0, error=str(e))


def mae(actual: npt.ArrayLike, predicted: npt.ArrayLike) -> ValueOrError:
    try:
        return ValueOrError(value=mean_absolute_error(actual, predicted))

    # ruff: noqa: BLE001 (Suppressed as we want to catch all exceptions here)
    except Exception as e:
        return ValueOrError(value=0, error=str(e))


def filter_by_equal_timestamps(a: Series, b: Series) -> tuple[Series, Series]:
    """
    filter_by_equal_timestamps will filter out samples from a and b
    that have the same timestamp.
    E.g. given
        a: (t1, a1), (t2,   a2),               (t3, a3)
        b: (t1, b1), (t2+d, b2), (t2 + X, b3),          (t4, b4)
    where
        d is less than (t2 - t1)
        X is > (t2 - t1)
    it returns
        a: (t1, a1), (t2,   a2)
        b: (t1, b1), (t2+d, b2)
    """

    filtered_a = []
    filterd_b = []

    idx_a, idx_b = 0, 0

    a_interval = a.samples[1].timestamp - a.samples[0].timestamp
    b_interval = b.samples[1].timestamp - b.samples[0].timestamp
    scrape_interval = min(a_interval, b_interval)

    while idx_a < len(a.samples) and idx_b < len(b.samples):
        if abs(b.samples[idx_b].timestamp - a.samples[idx_a].timestamp) < scrape_interval:
            filtered_a.append(a.samples[idx_a])
            filterd_b.append(b.samples[idx_b])
            idx_a += 1
            idx_b += 1
        elif a.samples[idx_a].timestamp < b.samples[idx_b].timestamp:
            idx_a += 1
        else:
            idx_b += 1

    return (
        Series.from_samples(a.query, filtered_a, a.labels),
        Series.from_samples(b.query, filterd_b, b.labels),
    )


class SeriesError(Exception):
    def __init__(self, query: str, expected: int, got: int):
        self.query = strip_query(query)
        self.expected = expected
        self.got = got

    def __str__(self) -> str:
        return f"Query: {self.query}\n\tExpected: {self.expected}\n\tGot: {self.got}"


def strip_query(query: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"\n", " ", query))


class Queryable(Protocol):
    # ruff: noqa: ARG002 (Suppressed as the arguments are intentionally unused in this context)
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
        logger.info("running query %s with step %s", strip_query(query), self.step)

        series = self.prom.custom_query_range(query=query, start_time=start, end_time=end, step=self.step)
        return [Series(query, s["values"], s["metric"]) for s in series]

    def kepler_build_info(self) -> list[str]:
        resp = self.prom.custom_query(query="kepler_exporter_build_info")
        build_info = [r["metric"] for r in resp]
        return [to_metric(b) for b in build_info]

    def kepler_node_info(self) -> list[str]:
        resp = self.prom.custom_query(query="kepler_node_info")
        labels = [r["metric"] for r in resp]
        return [to_metric(b) for b in labels]


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
        actual_query: str,
        predicted_query: str,
    ) -> Result:
        actual_series = self.single_series(actual_query, start, end)
        predicted_series = self.single_series(predicted_query, start, end)

        actual, predicted = filter_by_equal_timestamps(actual_series, predicted_series)
        actual_dropped = len(actual_series.samples) - len(actual.samples)
        predicted_dropped = len(predicted_series.samples) - len(predicted.samples)

        return Result(
            mse=mse(actual.values, predicted.values),
            mape=mape(actual.values, predicted.values),
            mae=mae(actual.values, predicted.values),
            actual_series=actual_series,
            predicted_series=predicted_series,
            actual_dropped=actual_dropped,
            predicted_dropped=predicted_dropped,
        )
