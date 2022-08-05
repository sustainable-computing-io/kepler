from estimator_client import Client
from estimator_test import generate_request, model_names
from estimator import SERVE_SOCKET
import numpy as np
import time
loads = range(10, 101, 10)
duration = 120

if __name__ == '__main__':
    client = Client(SERVE_SOCKET)
    for model_name in model_names:
        for load in loads:
            request_json = generate_request(model_name, load)
            start_time = time.time()
            client.make_request(request_json)
            elapsed_time = time.time() - start_time 
            output = '{},{},{}'.format(model_name, load, elapsed_time)
            print(output)
            time.sleep(1)