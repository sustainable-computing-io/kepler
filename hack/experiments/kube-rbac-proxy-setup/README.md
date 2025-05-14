# kube-rbac-proxy spike setup for openshift

## Deploy

1. Provision an Openshift cluster (remove fake-cpu-meter if rapl zones exist)

2. Apply power-monitor-* (except for power-monitor-sm.yaml) and kube-rbac-proxy-secret.yaml files

3. Apply test-* and confirm that kube-rbac-proxy successfully blocks test-wrong-job (default Service Account) and accepts test-correct-job (test-namespace ServiceAccount)

4. For UserWorkloadMonitoring test, deploy the ServiceMonitor (power-monitor-sm.yaml) and confirm that prometheus-user-workload is successfully scraping uwm

5. Note for uwm, it is essential you create the secret token for prometheus-user-workload service account in the openshift-user-workload-monitoring namespace
