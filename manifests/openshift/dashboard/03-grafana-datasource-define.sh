
#!/bin/sh

# Grant `cluster-monitoring-view` role to the `grafana-serviceaccount`
oc adm policy add-cluster-role-to-user cluster-monitoring-view -z grafana-serviceaccount

# Get bearer token for `grafana-serviceaccount`. 
export BEARER_TOKEN=$(oc serviceaccounts get-token grafana-serviceaccount -n monitoring)

# Deploy from updated manifest
envsubst < 03-grafana-datasource-UPDATETHIS.yaml | kubectl apply -f -