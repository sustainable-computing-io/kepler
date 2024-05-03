import paramiko
from  validator import config 
from typing import NamedTuple
from datetime import datetime



class ScriptResult(NamedTuple):
    start_time : datetime
    end_time : datetime

class Remote: 
    def __init__(self, config: config.Remote):
        self.host = config.host
        self.pkey = config.pkey
        self.user = config.user
        self.port =config.port
        self.password = config.password

        self.ssh_client = paramiko.SSHClient()
        self.ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    def __repr__(self):
        return f"<Remote {self.user}@{self.host}>"
    
    def connect(self):
        print(f"connecting -> {self.user}@{self.host}")

        if self.pkey is not None:
            pkey = paramiko.RSAKey.from_private_key_file(self.pkey)
            self.ssh_client.connect(
                hostname=self.host, port=self.port,
                username=self.user, pkey=pkey,
            )
        else:
            self.ssh_client.connect(
                hostname=self.host, port=self.port, 
                username=self.user, password=self.password)

    def copy(self, script_path, target_script):
        sftp_client = self.ssh_client.open_sftp()
        print(f"copying script {script_path} to remote - {target_script}")
        sftp_client.put(script_path, target_script)
        sftp_client.close()
        self.ssh_client.exec_command(f"chmod +x {target_script}")
        print(f"copying script {script_path} to remote - {target_script} - successfull")

    def run_script(self, script_path):
        self.connect()
        print(f"Running stress test ...")

        target_script = "/tmp/stress.sh"
        self.copy(script_path, target_script)

        start_time = datetime.now()
        _, stdout, stderr = self.ssh_client.exec_command(target_script)
        exit_status = stdout.channel.recv_exit_status()
        end_time = datetime.now()
        self.ssh_client.close()

        if exit_status == 0:
            print("script execution successful")
        else:
            print("script execution failed")
            
        print("logs for stress test:")
        for line in stdout:
            print("  ", line.strip())

        print("stderr output:")
        for line in stderr:
            print(" ", line.strip())

        return ScriptResult(
            start_time=start_time,
            end_time=end_time,
        )
