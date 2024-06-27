import datetime

import click
from prometheus_api_client.utils import parse_datetime


class DateTime(click.ParamType):
    name = "datetime"

    def convert(self, value, param, ctx):
        try:
            date, time = value.split(" ")
        except ValueError:
            time = value
            date = datetime.date.today()

        dt = parse_datetime(f"{date} {time}")
        if not dt:
            self.fail(
                "expected datetime format conversion, got " f"{value!r}",
                param,
                ctx,
            )

        return dt
