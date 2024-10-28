import datetime
import json
from typing import Any


class Value:
    def __init__(self, mse: str = "", mape: str = "", mae: str = "", status: str = ""):
        self.mse = mse
        self.mape = mape
        self.mae = mae
        self.status = status

    def to_dict(self):
        return {"mse": self.mse, "mape": self.mape, "mae": self.mae, "status": self.status}

    def __repr__(self):
        return f"Value(mse='{self.mse}', mape='{self.mape}', mae='{self.mae}', status='{self.status}')"


class Result:
    def __init__(self, metric_name: str, value: dict[str, Any]):
        if value is None:
            value = {}
        self.metric_name = metric_name
        self.value = Value(**value)

    def to_dict(self):
        return {"metric-name": self.metric_name, "value": self.value.to_dict()}

    def __repr__(self):
        return f"Result(metric_name='{self.metric_name}', value={self.value})"


class JsonTemplate:
    def __init__(
        self,
        build_info: list[Any],
        node_info: list[Any],
        machine_specs: list[Any],
        results: list[dict[str, Any]],
    ):
        self.build_info = build_info
        self.node_info = node_info
        self.machine_specs = machine_specs
        self.results = []
        for res in results:
            for key, value in res.items():
                self.results.append(Result(key, value))

    def to_dict(self):
        return {
            "build_info": self.build_info,
            "node_info": self.node_info,
            "machine_specs": self.machine_specs,
            "results": [res.to_dict() for res in self.results],
        }

    def __repr__(self):
        return (
            f"JsonTemplate('build_info={self.build_info}, "
            f"node_info={self.node_info}, machine_spec={self.machine_specs}, "
            f"result={self.results})"
        )


class CustomEncoder(json.JSONEncoder):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

    def default(self, o):
        if hasattr(o, "_asdict"):
            return o._asdict()

        if hasattr(o, "to_dict"):
            return o.to_dict()

        if type(o) == datetime.datetime:
            return o.isoformat()

        if type(o).__name__ == "bool":
            return str(o).lower()

        return super().default(o)
