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
        file_path: str,
        build_info: list[Any],
        node_info: list[Any],
        machine_spec: list[Any],
        result: list[dict[str, Any]],
    ):
        if build_info is None:
            build_info = []
        if node_info is None:
            node_info = []
        if machine_spec is None:
            machine_spec = []
        if result is None:
            result = []

        self.file_path = file_path
        self.build_info = build_info
        self.node_info = node_info
        self.machine_spec = machine_spec
        self.result = []
        for res in result:
            for key, value in res.items():
                self.result.append(Result(key, value))

    def to_dict(self):
        return {
            "file_path": self.file_path,
            "build_info": self.build_info,
            "node_info": self.node_info,
            "machine_spec": self.machine_spec,
            "result": [res.to_dict() for res in self.result],
        }

    def __repr__(self):
        return (
            f"JsonTemplate(file_path='{self.file_path}', build_info={self.build_info}, "
            f"node_info={self.node_info}, machine_spec={self.machine_spec}, "
            f"result={self.result})"
        )


class CustomEncoder(json.JSONEncoder):
    def default(self, obj):
        if hasattr(obj, "to_dict"):
            return obj.to_dict()
        return super().default(obj)
