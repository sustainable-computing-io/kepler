# Dashboard versions for Kepler on OpenShift

We have found that with some installations Grafana does not scale well to the data Kepler is providing so the dashboard is either slow to render of doesn't show any data. 
For that reeason to allow users to visualise Kepler data with the demo dashboard we are now providing two versions.

1. The original dashboard that will visualise all Kepler data
2. A version of the dashboard that uses topk to reduce the data being visualised and also changing the default Grafana namespace to the kepler namespace

# Enabling Dashboard for Kepler on OpenShift

The following cmd will:
- Enable OpenShift User Workload Monitoring
- Deploy Grafana operator
- Create and configure Grafana instance for Kepler
- Define Prometheus datasource
- Define Grafana dashboard

It should be run from the top level of the repository

To install the standard dashboard use this script


```bash
manifests/config/dashboard/deploy-grafana.sh
```

To install the topk dashboard use this script

```bash
manifests/config/dashboard/deploy-grafana-topk.sh
```