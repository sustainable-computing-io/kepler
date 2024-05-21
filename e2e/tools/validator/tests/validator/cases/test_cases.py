from validator.cases import Cases
from validator.config import Prometheus, VM

basic_raw_prom_queries = [
        {
            "expected_query": "rate(kepler_process_package_joules_total{{job='metal', pid='{vm_pid}', mode='dynamic'}}[{interval}])",
            "actual_query": "rate(kepler_node_platform_joules_total{{job='vm'}}[{interval}])",
        },
        {
            "expected_query": "rate(kepler_process_platform_joules_total{{job='metal', pid='{vm_pid}', mode='dynamic'}}[{interval}])",
            "actual_query": "rate(kepler_node_platform_joules_total{{job='vm'}}[{interval}])",
        },
    ]


def test_load_cases_basic():
    prom_config = Prometheus(
        url="http://localhost:9090",
        interval='12s',
        step='3s'
    )
    
    vm_config = VM(
        pid=17310
    )

    sample_test_cases = Cases(vm_config, prom_config)
    # modify prom queries
    sample_test_cases.raw_prom_queries = basic_raw_prom_queries
    refined_test_cases = sample_test_cases.load_test_cases().test_cases
    print(refined_test_cases)
    assert refined_test_cases[0].expected_query == \
        "rate(kepler_process_package_joules_total{job='metal', pid='17310', mode='dynamic'}[12s])"
    assert refined_test_cases[0].actual_query == \
        "rate(kepler_node_platform_joules_total{job='vm'}[12s])"
    assert refined_test_cases[1].expected_query == \
        "rate(kepler_process_platform_joules_total{job='metal', pid='17310', mode='dynamic'}[12s])"
