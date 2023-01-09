#!/bin/bash
kubectl apply -f role.yaml

SERVICE_ACCOUNT=kubelet-visitor
SECRET=kubelet-visitor-token
# Extract the Bearer token from the Secret and decode
TOKEN=$(kubectl get secret ${SECRET} -o json | jq -Mr '.data.token' | base64 -d)
echo -n ${TOKEN} > /tmp/token
# Extract, decode and write the ca.crt to a temporary location
kubectl get secret ${SECRET} -o json | jq -Mr '.data["ca.crt"]' | base64 -d > /tmp/ca.crt

# run these to sanity check the token
#curl -s https://127.0.0.1:10250/pods  --header "Authorization: Bearer $TOKEN" --cacert /tmp/ca.crt > /tmp/output.json
#curl -s https://127.0.0.1:10250/metrics/resource  --header "Authorization: Bearer $TOKEN" --cacert /tmp/ca.crt > /tmp/res.json

# save the token for kepler
mkdir -p /var/run/secrets/kubernetes.io/serviceaccount/
cp /tmp/token /var/run/secrets/kubernetes.io/serviceaccount/
