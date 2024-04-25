import random
import time
import signal
import sys
from prometheus_client import start_http_server, Gauge
import configparser


# Create a Prometheus gauge metric
gauge = Gauge('mock_acpi_power1_average', 'Random walk acpi power value value')

CONFIG_FILE = '/var/mock-acpi-config/mock-acpi.ini'

POWER_FILE = 'power1_average'


def signal_handler(signum, frame):
    print(f"Gracefully shutting down after receiving signal {signum}")
    sys.exit(0)


def update(file, value):
    # Update Prometheus metric
    gauge.set(value)

    # Update power1_average
    try:
        file.seek(0)
        file.write(str(value))
        file.flush()
    except Exception as e:
        print(e)


def walk(config):
    power_path = config.get('mock.acpi', 'power_path')
    file = f"{power_path}/{POWER_FILE}"
    print(f"writing power data to {file}")
    power1_average = open(file, "w+")
    start_value = config.getint('mock.acpi', 'start_value')
    current_value = start_value
    boost = 0

    while True:
        max_step = config.getint('mock.acpi', 'max_step')
        sleep_time = config.getfloat('mock.acpi', 'sleep_msec')
        boost = config.getint('mock.acpi', 'boost')
        step = random.randint(-max_step, max_step)
        current_value += step
        current_value += boost
        update(power1_average, current_value)
        current_value -= boost
        time.sleep(sleep_time / 1000)
        config.read(CONFIG_FILE)


if __name__ == '__main__':

    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    config = configparser.ConfigParser()
    config.read(CONFIG_FILE)
    port = config.getint('mock.acpi', 'metrics_port')

    # Start the Prometheus HTTP server
    start_http_server(port)

    walk(config)
