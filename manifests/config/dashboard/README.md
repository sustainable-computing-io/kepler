# Enabling Dashboard for Kepler on OpenShift

The following cmd will:
- Enable OpenShift User Workload Monitoring
- Deploy Grafana operator
- Create and configure Grafana instance for Kepler
- Define Prometheus datasource
- Define Grafana dashboard

It should be run from the top level of the repository
```bash
manifests/config/dashboard/deploy-grafana.sh
```
