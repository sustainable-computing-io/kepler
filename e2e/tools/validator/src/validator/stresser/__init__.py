import logging
from datetime import datetime
from typing import NamedTuple

import paramiko

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
