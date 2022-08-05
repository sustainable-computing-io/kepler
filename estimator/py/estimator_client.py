from estimator_test import generate_request, model_names
from estimator import SERVE_SOCKET
import sys
import socket
import json
import time

import cProfile

class Client:
    def __init__(self, socket_path):
        self.socket_path = socket_path

    def make_request(self, request_json):
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.connect(self.socket_path)
        data = json.dumps(request_json)
        s.send(data.encode())
        data = b''
        while True:
            shunk = s.recv(1024).strip()
            data += shunk
            if shunk is None or shunk.decode()[-1] == '}':
                break
        decoded_data = data.decode()
        s.close()
        return decoded_data

if __name__ == '__main__':
    client = Client(SERVE_SOCKET)
    request_json = generate_request(model_names[0], 2)
    res = client.make_request(request_json)
    print(res)