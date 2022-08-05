# to test only py part
# 

from estimator import handle_request, load_all_models
import json
import cProfile

values =  [[0.0, 0.0,
            10261888.000000002,
            657541352.0,
            271351808.0,
            8672664.0,
            657541352.0,
            315585868416.0,
            797468706528.0,
            21.008000000000003]]

metrics = ['curr_bytes_read',
            'curr_bytes_writes',
            'curr_cache_misses',
            'curr_cgroupfs_cpu_usage_us',
            'curr_cgroupfs_memory_usage_bytes',
            'curr_cgroupfs_system_cpu_usage_us',
            'curr_cgroupfs_user_cpu_usage_us',
            'curr_cpu_cycles',
            'curr_cpu_instructions',
            'curr_cpu_time']
power_dict = {
    'core_power': [10],
    'dram_power': [3],
    'gpu_power': [0],
    'other_power': [0]
    }

model_names = [None, 'GradientBoostingRegressor_10', 'Linear Regression_10', 'Polynomial Regression_10', 'CorrRatio']

def generate_request(model_name, n=1):
    request_json = dict() 
    if model_name is not None:
        request_json['model_name'] = model_name
    request_json['metrics'] = metrics
    request_json['values'] = [values[0]]*n
    request_json.update(power_dict)
    return request_json

if __name__ == '__main__':
    model_df = load_all_models()
    for n in range(10, 101, 10):
        for request_json in [generate_request(model_name,n) for model_name in model_names]:
                data = json.dumps(request_json)
                model_name = 'auto' if 'model_name' not in request_json else request_json['model_name']
                print(model_name, n)
                cProfile.run('handle_request(model_df, data)')