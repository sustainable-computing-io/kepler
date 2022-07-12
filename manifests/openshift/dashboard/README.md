# Enabling Dashboard for Kepler on OpenShift

- Enable OpenShift User Workload Monitoring
```bash
# Enable OpenShift user workload monitoring
oc apply -f 00-openshift-monitoring-user-projects.yaml

# Setup service monitor policy
oc apply -f 00-servicemonitoring-kepler.yaml
```
- Deploy Grafana operator
```bash
# Install Grafana community operator
oc apply -f 01-grafana-operator.yaml
```
- Create and configure Grafana instance for Kepler
```bash
# Create Grafana Instance
oc apply -f 02-grafana-instance.yaml

# Define Prometheus datasource
03-grafana-datasource-define.sh

# Define Grafana dashboard
oc apply -f 04-grafana-dashboard.yaml
```