# pv-gc

Sample controller. Garbage collect persistent volumes in `Released` state.

## Run in kind

- Create kind cluster with next config:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
```
```bash
kind create cluster --config=kind.yaml
```

- Build docker image
```bash
docker build pv-gc:0.0.1 .
```

- Load image in the kind cluster
```bash
kind load docker-image pv-gc:0.0.1
```

- Deploy to cluster
```bash
kubectl --context kind-kind create -f deploy/deployment.yaml
```

- Verify
```bash
kubectl --context kind-kind get po
```