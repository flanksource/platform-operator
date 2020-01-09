# Platform Operator

Platform Operator is Kubernetes operator designed to be run in a multi-tenanted environment.

Current features:

* Auto-Delete: cleanup namespaces after a certain expiry period by labeleling the namespace with `auto-delete`

## Install

Run:

```console
make deploy
cd config/manager && kustomize edit set image controller=controller:latest
kustomize build config/default | kubectl apply -f -
namespace/platform-operator-system created
role.rbac.authorization.k8s.io/platform-operator-leader-election-role created
clusterrole.rbac.authorization.k8s.io/platform-operator-manager-role created
clusterrole.rbac.authorization.k8s.io/platform-operator-proxy-role created
rolebinding.rbac.authorization.k8s.io/platform-operator-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/platform-operator-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/platform-operator-proxy-rolebinding created
service/platform-operator-controller-manager-metrics-service created
deployment.apps/platform-operator-controller-manager created
```

This command use `kustomize` to build the manifests. Once ready the manifests are applied to the cluster and the operator starts.
