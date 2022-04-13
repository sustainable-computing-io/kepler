# Install Kepler on Kubernetes
```bash
kubectl apply -f deployment.yaml
```

# Install Kepler on OpenShift
```bash
kubectl apply -f openshift-kernel-devel.yaml
kubectl apply -f openshift-scc.yaml
kubectl apply -f openshift-deployment.yaml
```
