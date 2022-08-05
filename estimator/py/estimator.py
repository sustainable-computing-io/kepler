# predict.py <model name> <feature values>
# load model, scaler, and metadata from model folder ../data/model/<model name>
# apply scaler and model to metadata
import pandas as pd
import numpy as np
import json
import pickle
import keras
import os

dirname = os.path.dirname(__file__)

MODEL_FOLDER = os.path.join(dirname, '../model')
METADATA_FILENAME = 'metadata.json'
SCALER_FILENAME = 'scaler.pkl'

SERVE_SOCKET = '/tmp/estimator.sock'

###############################################
# power request 

class PowerRequest():
    def __init__(self, metrics, values, model_name="", core_power=[], dram_power=[], gpu_power=[], other_power=[]):
        self.model_name = model_name
        self.datapoint = pd.DataFrame(values, columns=metrics)
        self.core_power = core_power
        self.dram_power = dram_power
        self.gpu_power = gpu_power
        self.other_power = other_power

###############################################
# load data

def _modelpath(model_name):
    return "{}/{}/".format(MODEL_FOLDER, model_name)

def load_metadata(model_name):
    metadata_file = _modelpath(model_name) + METADATA_FILENAME
    try:
        with open(metadata_file) as f:
            metadata = json.load(f)
    except Exception as e:
        print(e)
        return None
    return metadata

def load_model_by_pickle(model_name, model_filename):
    model_file = _modelpath(model_name) + model_filename
    try:
        with open(model_file, 'rb') as f:
            model = pickle.load(f)
    except Exception as e:
        print(e)
        return None
    return model

def load_model_by_keras(model_name, model_filename):
    try:
        model = keras.models.load_model(model_filename)
    except Exception as e:
        print(e)
        return None
    return model

def load_model_by_json(model_name, model_filename):
    model_file = _modelpath(model_name) + model_filename
    try:
        with open(model_file) as f:
            model = json.load(f)
    except Exception as e:
        print(e)
        return None
    return model
###############################################

###############################################
# define model
def transform_and_predict(model, request):
    msg = ""
    try:
        x_values = request.datapoint[model.features].values
        normalized_x = model.scaler.transform(x_values)
        for fe in model.fe_list:
            if fe is None:
                continue
            normalized_x = fe.transform(normalized_x)
        y = model.model.predict(normalized_x)
        y = list(y)
    except Exception as e:
        msg = '{}\n'.format(e)
        y = []
    return y, msg

class ScikitModel():
    def __init__(self, model_name, model_file, features, fe_files):
        self.name = model_name
        self.features = features
        self.scaler = load_model_by_pickle(model_name, SCALER_FILENAME)
        self.model = load_model_by_pickle(model_name, model_file)
        self.fe_list = []
        for fe_filename in fe_files:
            self.fe_list += [load_model_by_pickle(model_name, fe_filename)]

    def get_power(self, request):
        return transform_and_predict(self, request)

class KerasModel():
    def __init__(self, model_name, model_file, features, fe_files):
        self.name = model_name
        self.features = features
        self.scaler = load_model_by_pickle(model_name, SCALER_FILENAME)
        self.model = load_model_by_keras(model_name, model_file)
        self.fe_list = []
        for fe_filename in fe_files:
            self.fe_list += [load_model_by_pickle(model_name, fe_filename)]

    def get_power(self, request):
        return transform_and_predict(self, request)


class RatioModel():
    def __init__(self, model_name, model_file, features, fe_files):
        self.name = model_name
        self.features = features
        self.model = load_model_by_json(model_name, model_file)
        self.power_components = ['core', 'dram', 'gpu', 'other']
    
    def get_power(self, request):
        msg = ""
        try:
            df = request.datapoint[self.features]
            if len(df) == 1:
                total_power = 0
                for component in self.power_components:
                    total_power += np.sum(getattr(request, '{}_power'.format(component)))
                return [total_power], msg
            sum_wl_stat = pd.DataFrame([df.sum().values], columns=df.columns, index=df.index)
            ratio_df = df.join(sum_wl_stat, rsuffix='_sum')
            output_df = pd.DataFrame()
            for component in self.power_components:
                for metric in self.features:
                    ratio_df[metric +'_{}_ratio'.format(component)] = ratio_df[metric]/ratio_df[metric+'_sum']*self.model['{}_score'.format(component)][metric]
                sum_ratio_df = ratio_df[[col for col in ratio_df if '{}_ratio'.format(component) in col]].sum(axis=1)
                total_power = getattr(request, '{}_power'.format(component))
                output_df[component] = sum_ratio_df*total_power
            y = list(output_df.sum(axis=1).values.squeeze())
        except Exception as e:
            msg = '{}\n'.format(e)
            y = []
        return y, msg

###############################################
# model wrapper

MODELCLASS = {
    'scikit': ScikitModel,
    'keras': KerasModel,
    'ratio': RatioModel
}

class Model():
    def __init__(self, model_class, model_name, model_file, features, fe_files=[], mae=None, mse=None, mae_val=None, mse_val=None, abs_model=None):
        self.model_name = model_name
        self.model = MODELCLASS[model_class](model_name, model_file, features, fe_files)
        self.mae = mae
        self.mse = mse
        self.abs_model = abs_model

    def get_power(self, request):
        return self.model.get_power(request)

def init_model(model_name):
    metadata = load_metadata(model_name)
    if metadata is not None:
        metadata_str = json.dumps(metadata)
        try: 
            model = json.loads(metadata_str, object_hook = lambda d : Model(**d))
            return model
        except Exception as e:
            print(e)
            return None
    return None


def load_all_models():
    model_names = [f for f in os.listdir(MODEL_FOLDER) if not os.path.isfile(os.path.join(MODEL_FOLDER,f))]
    print("Load models:", model_names)
    items = []
    for model_name in model_names:
        model = init_model(model_name)
        if model is not None:
            item = {'name': model.model_name, 'model': model, 'mae': model.mae, 'mse': model.mse}
            items += [item]
    model_df = pd.DataFrame(items)
    model_df = model_df.sort_values([DEFAULT_ERROR_KEY])
    return model_df

###############################################
# serve

import sys
import socket
import signal

DEFAULT_ERROR_KEY = 'mae'

def handle_request(model_df, data):
    try:
        power_request = json.loads(data, object_hook = lambda d : PowerRequest(**d))
    except Exception as e:
        msg = 'fail to handle request: {}'.format(e)
        return {"powers": [], "msg": msg}

    if len(model_df) > 0:
        best_available_model = model_df.iloc[0]['model']
    else:
        best_available_model = None
    if power_request.model_name == "":
        model = best_available_model
    else:
        selected = model_df[model_df['name']==power_request.model_name]
        if len(selected) == 0:
            print('cannnot find model: {}, use best available model'.format(power_request.model_name))
            model = best_available_model
        else:
            model = selected.iloc[0]['model']
    if model is not None:
        print('Estimator model: ', model.model_name)
        powers, msg = model.get_power(power_request)
        return {"powers": powers, "msg": msg}
    else:
        return {"powers": [], "msg": "no model to apply"}

class EstimatorServer:
    def __init__(self, socket_path):
        self.socket_path = socket_path
        self.model_df = load_all_models()
    
    def start(self):
        s = self.socket = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        s.bind(self.socket_path)
        s.listen(1)
        try:
            while True:
                connection, address = s.accept()
                self.accepted(connection, address)
        finally:
            os.remove(self.socket_path)
            sys.stdout.write("close socket\n")

    def accepted(self, connection, address):
        data = b''
        while True:
            shunk = connection.recv(1024).strip()
            data += shunk
            if shunk is None or shunk.decode()[-1] == '}':
                break
        decoded_data = data.decode()
        y = handle_request(self.model_df, decoded_data)
        response = json.dumps(y)
        connection.send(response.encode())

def clean_socket():
    print("clean socket")
    if os.path.exists(SERVE_SOCKET):
        os.unlink(SERVE_SOCKET)

def sig_handler(signum, frame) -> None:
    clean_socket()
    sys.exit(1)

import argparse

if __name__ == '__main__':
    clean_socket()
    signal.signal(signal.SIGTERM, sig_handler)
    try:
        parser = argparse.ArgumentParser()
        parser.add_argument('-e', '--err',
                            required=False,
                            type=str,
                            default='mae', 
                            metavar="<error metric>",
                            help="Error metric for determining the model with minimum error value" )
        args = parser.parse_args()
        DEFAULT_ERROR_KEY = args.err
        server = EstimatorServer(SERVE_SOCKET)
        server.start()
    finally:
        clean_socket()