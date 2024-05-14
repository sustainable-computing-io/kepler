import subprocess
import signal
import sys
from prometheus_client import start_http_server, Gauge

# Create a Prometheus gauge metric
gauge = Gauge('turbostat_cpu_freq', 'CPU Frequency as reported by turbostat', ["cpu", "core"])


def signal_handler(signum, frame):
    print(f"Gracefully shutting down after receiving signal {signum}")
    sys.exit(0)


def parse_turbostat_output(line):
    """
    Parse a single line of output from the turbostat command.
    """
    values = line.split()
    if values[0] != "Core":
        gauge.labels(core=values[0], cpu=values[1]).set(values[2])



def run_turbostat():
    """
    Run the turbostat command and continuously read its output.
    """
    # turbostat -s Core,CPU,Avg_MHz --quiet
    process = subprocess.Popen(['turbostat', '-s', 'Core,CPU,Avg_MHz', '--quiet'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, universal_newlines=True)

    while True:
        # Read a line of output from the process
        line = process.stdout.readline().strip()

        if line:
            # Parse the line of output
            parse_turbostat_output(line)
        else:
            # If no output, check if the process has terminated
            return_code = process.poll()
            if return_code is not None:
                # The process has terminated, so exit the loop
                break


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)
    try:
        start_http_server(8001)
        run_turbostat()
    except KeyboardInterrupt:
        # Exit the program if the user presses Ctrl+C
        sys.exit(0)

