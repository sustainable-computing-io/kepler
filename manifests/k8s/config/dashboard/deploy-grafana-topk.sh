#!/bin/sh

# Applying dashboard/kustomization.yaml does the following:
## Enables dashboard for Kepler on OpenShift
## Setup service monitor policy
## Installs Grafana community operator
## Creates Grafana instance

oc apply --kustomize $(pwd)/manifests/config/dashboard
while ! oc get grafana --all-namespaces
do
    echo waiting for grafana custom resource definition to register
    sleep 5
done
oc apply -f $(pwd)/manifests/config/dashboard/02-grafana-instance.yaml
oc apply -f $(pwd)/manifests/config/dashboard/02-grafana-sa.yaml
oc apply -f $(pwd)/manifests/config/dashboard/03-grafana-sa-token-secret.yaml

SERVICE_ACCOUNT=grafana-serviceaccount
SECRET=grafana-sa-token

while ! oc get serviceaccount $SERVICE_ACCOUNT -n kepler
do
    sleep 2
done
# Define Prometheus datasource
oc adm policy add-cluster-role-to-user cluster-monitoring-view -z $SERVICE_ACCOUNT -n kepler


export BEARER_TOKEN=$(oc get secret ${SECRET} -o json -n kepler | jq -Mr '.data.token' | base64 -d) || or true
# Get bearer token for `grafana-serviceaccount`
while [ -z "$BEARER_TOKEN" ]
do
    echo waiting for service account token
    export BEARER_TOKEN=$(oc get secret ${SECRET} -o json -n kepler | jq -Mr '.data.token' | base64 -d) || or true
    sleep 1
done
echo service account token is populated, will now create grafana datasource

while ! oc get grafanadatasource --all-namespaces;
do
    sleep 1
    echo waiting for grafanadatasource custom resource definition to register
done

# Deploy from updated manifest
envsubst < $(pwd)/manifests/config/dashboard/03-grafana-datasource-UPDATETHIS.yaml | oc apply -f -

# Define Grafana dashboard
while ! oc get grafanadashboard --all-namespaces;
do
    sleep 1
    echo waiting for grafandashboard custom resource definition to register
done

oc apply -f $(pwd)/manifests/config/dashboard/04-grafana-dashboard-topk.yaml