import logging
from datetime import datetime
from typing import NamedTuple, List, Iterable

import paramiko
import os
import subprocess
import time
import docker
import psutil


from validator import config

logger = logging.getLogger(__name__)


class ScriptResult(NamedTuple):
    start_time: datetime
    end_time: datetime


class RunResult(NamedTuple):
    stdout: str
    stderr: str
    exit_code: int


class Remote:
    def __init__(self, config: config.Remote):
        self.host = config.host
        self.pkey = config.pkey
        self.user = config.user
        self.port = config.port
        self.password = config.password

        self.ssh_client = paramiko.SSHClient()
        self.ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    def __repr__(self):
        return f"<Remote {self.user}@{self.host}>"

    def connect(self):
        logger.info("connecting -> %s@%s", self.user, self.host)

        if self.pkey:
            logger.debug("using pkey to connect")
            pkey = paramiko.RSAKey.from_private_key_file(self.pkey)
            self.ssh_client.connect(
                hostname=self.host,
                port=self.port,
                username=self.user,
                pkey=pkey,
            )
        else:
            logger.debug("using user/password to connect")
            self.ssh_client.connect(
                hostname=self.host,
                port=self.port,
                username=self.user,
                password=self.password,
            )

    def copy(self, script_path, target_script):
        sftp_client = self.ssh_client.open_sftp()
        logger.info("copying script %s to remote - %s", script_path, target_script)
        sftp_client.put(script_path, target_script)
        sftp_client.close()
        self.ssh_client.exec_command(f"chmod +x {target_script}")
        logger.info("copying script %s to remote - %s - successful", script_path, target_script)

    def run_script(self, script_path: str, target_script: str, **kwargs) -> ScriptResult:
        self.connect()

        cli_options = " ".join([f"-{k} {v}" for k, v in kwargs.items()]) if kwargs else ""
        command = f"{target_script} {cli_options}"
        self.copy(script_path, target_script)
        logger.info("Running command %s ...", command)
        # ruff: noqa: DTZ005 (Suppressed non-time-zone aware object creation as it is not necessary for this use case)
        start_time = datetime.now()
        _, stdout, stderr = self.ssh_client.exec_command(command)

        # ruff: noqa: T201 (Suppressed as printing is intentional and necessary in this context)
        print("stdout output:")
        for line in stdout:
            print(" ┊ ", line.strip())

        exit_status = stdout.channel.recv_exit_status()
        end_time = datetime.now()
        self.ssh_client.close()

        # ruff: noqa: T201 (Suppressed as printing is intentional and necessary in this context)
        # report any stderr if there is after stdout
        print("\nstderr output:")
        for line in stderr:
            print(" ┊ ", line.strip())
        print("‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\n\n")

        if exit_status != 0:
            logger.warning("script execution failed")
        else:
            logger.info("script execution successful")

        return ScriptResult(
            start_time=start_time,
            end_time=end_time,
        )

    def run(self, cmd: str, *args) -> RunResult:
        self.connect()
        logger.info("running command: %s ", " ".join([cmd, *args]))

        _, stdout, stderr = self.ssh_client.exec_command(" ".join([cmd, *args]))
        exit_status = stdout.channel.recv_exit_status()
        self.ssh_client.close()

        return RunResult(
            stdout=stdout.read().decode("ascii").strip("\n"),
            stderr=stderr.read().decode("ascii").strip("\n"),
            exit_code=exit_status,
        )


class ProcessOutput(NamedTuple):
    script_result: ScriptResult
    relevant_pids: Iterable[str]


class ContainerOutput(NamedTuple):
    ScriptResult: ScriptResult
    container_id: str


def return_child_pids(parent_pid: int) -> List[int]:
    try:
        parent_process = psutil.Process(parent_pid)
        children_processes = parent_process.children(recursive=True)
        return [child_process.pid for child_process in children_processes]
    except psutil.NoSuchProcess:
        return []

def retrieve_time_interval_from_log(time_interval_filepath):
    start_time = None
    end_time = None
    with open(file=time_interval_filepath, mode="r") as f:
        for line in f.readlines():
            if line.startswith("Stress Start Time:"):
                start_timestamp = (line.split(":")[-1]).strip()
                start_time = datetime.fromtimestamp(float(start_timestamp))
            if line.startswith("Stress End Time:"):
                end_timestamp = (line.split(":")[-1]).strip()
                end_time = datetime.fromtimestamp(float(end_timestamp))
    return start_time, end_time


class Local:
    def __init__(self, config: config.Local):
        self.load_curve = config.load_curve
        self.iterations = config.iterations
        self.mount_dir = config.mount_dir
        self.time_range_log = os.path.join(self.mount_dir, "time_interval.log")
        stresser_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../"))
        self.stresser_script =os.path.join(stresser_dir, "scripts", "targeted_stresser.sh")

    def __repr__(self):
        return f"<Local> Local Stresser\nLoad Curve: {self.load_curve}"
    
    def stress(self):
        logger.info("stressing in local mode")
        command = f"{self.stresser_script} -g -d {self.mount_dir} -l {self.load_curve} -n {self.iterations}"
        logger.info("running stress command -> %s", command)
        print(command)
        result = subprocess.run(command, shell=True, check=True, text=True, capture_output=True)
        print(f"Output:\n{result.stdout}")
        status_code = result.returncode
        if status_code != 0:
            logger.error("stresser command failed -> %s", command)
            raise StresserError(
                script_exit_code=status_code,
                message="status code is non zero"
            )
        start_time, end_time = retrieve_time_interval_from_log(self.time_range_log)
        if start_time is None or end_time is None:
            logger.error("start time or end time is empty")
            raise StresserError(
                start_time=start_time,
                end_time=end_time,
                message="start time or end time is empty"
            )
        return ScriptResult(
            start_time=start_time,
            end_time=end_time
        )


class Process(Local):
    def __init__(self, config: config.Local):
        super().__init__(config)
        self.isolated_cpu = config.isolated_cpu

    def __repr__(self):
        return f"<Local> Process Stresser\nLoad Curve: {self.load_curve}"

    def stress(self):
        logger.info("stressing in process mode -> %s", self.isolated_cpu)
        command = f"{self.stresser_script} -r '{self.isolated_cpu}' -d {self.mount_dir} -l {self.load_curve} -n {self.iterations}"
        logger.info("running stress command -> %s", command)
        target_popen = subprocess.Popen(command, shell=True)
        time.sleep(1)
        target_process_pid = target_popen.pid
        all_child_pids = set([target_process_pid])
        while target_popen.poll() is None:
            child_pids = return_child_pids(target_process_pid)
            all_child_pids = all_child_pids.union(child_pids)
            time.sleep(1)
        logger.info("captured pids -> %s", all_child_pids)
        print(f"captured pids: {all_child_pids}")
        status_code = target_popen.returncode
        if status_code != 0:
            logger.error("stresser command failed -> %s", command)
            raise StresserError(
                script_exit_code=status_code,
                message="status code is non zero"
            )

        start_time, end_time = retrieve_time_interval_from_log(self.time_range_log)

        if start_time is None or end_time is None:
            logger.error("start time or end time is empty")
            raise StresserError(
                start_time=start_time,
                end_time=end_time,
                message="start time or end time is empty"
            )
        
        return ProcessOutput(
            script_result=ScriptResult(
                start_time=start_time,
                end_time=end_time
            ),
            relevant_pids=all_child_pids
        )


class Container(Local):
    def __init__(self, config: config.Local):
        super().__init__(config)
        self.isolated_cpu = config.isolated_cpu
        self.container_name = config.container_name
        self.client = docker.from_env()

    def __repr__(self):
        return f"<Local> Container Stresser\nLoad Curve: {self.load_curve}"

    def stress(self) -> ContainerOutput:
        logger.info("stressing in container mode -> %s", self.isolated_cpu)
        image = "fedora:latest"
        command = f"bash -c 'dnf update -y && dnf install -y stress-ng && bash /app/stresser_script.sh \
                    -r \"{self.isolated_cpu}\" -d \"{self.mount_dir}\" \
                    -l \"{self.load_curve}\" -n \"{self.iterations}\"'"
        self.client.images.pull(image)
        stress_container = self.client.containers.run(
            image=image,
            name=self.container_name,
            command=command,
            volumes={
                self.stresser_script: {'bind': '/app/stresser_script.sh', 'mode': 'ro'},
                self.mount_dir :{'bind': self.mount_dir, 'mode': 'rw'}
            },
            remove=False,
            detach=True

        )
        id = stress_container.id
        logger.info("captured container id -> %s", id)
        print(f"captured container id: {id}")
        status_map = stress_container.wait()
        container_logs = stress_container.logs().decode("utf-8")
        logger.info("container logs ->\n%s", container_logs)
        print(f"container logs:\n{container_logs}")
        stress_container.remove()
        print(status_map)
        
        status_code = status_map["StatusCode"]
        if status_map["StatusCode"] != 0:
            logger.error("stresser command failed -> %s", command)
            raise StresserError(
                script_exit_code=status_code,
                message="status code is non zero"
            )

        start_time, end_time = retrieve_time_interval_from_log(self.time_range_log)

        if not start_time or not end_time:
            logger.error("start time or end time is empty")
            raise StresserError(
                start_time=start_time,
                end_time=end_time,
                message="start time or end time is empty"
            )

        logger.info("Stress Start Time: %s\nStress End Time: %s", start_time, end_time)
        print(start_time, end_time)
        return ContainerOutput(
            script_result=ScriptResult(
                start_time=start_time,
                end_time=end_time
            ),
            container_id=id,
        )


class StresserError(Exception):
    def __init__(self, start_time=None, end_time=None, script_exit_code=0, message=""):
        super().__init__(message)
        self.start_time = start_time
        self.end_time = end_time
        self.script_exit_code = script_exit_code

    def __str__(self) -> str:
        base_message = super().__str__()
        return f"Start Time: {self.start_time}\nEnd Time: {self.end_time}\nScript Code: {self.script_exit_code}\nMessage: {base_message}"