# Platform Operator

Platform Operator is Kubernetes operator designed to be run in a multi-tenanted environment.

Current features:

* Auto-Delete: cleanup namespaces after a certain expiry period by labeleling the namespace with `auto-delete`.
* ClusterResourceQuota: allows quotas to be enforced across the entire cluster.

## Install

Run:

```console
make deploy TAG=0.0.1
cd config/manager && kustomize edit set image controller=controller:0.0.1
kustomize build config/default | kubectl apply -f -
namespace/platform-operator-system configured
customresourcedefinition.apiextensions.k8s.io/clusterresourcequotas.platform.flanksource.com configured
role.rbac.authorization.k8s.io/platform-operator-leader-election-role configured
clusterrole.rbac.authorization.k8s.io/platform-operator-manager-role configured
clusterrole.rbac.authorization.k8s.io/platform-operator-proxy-role configured
rolebinding.rbac.authorization.k8s.io/platform-operator-leader-election-rolebinding configured
clusterrolebinding.rbac.authorization.k8s.io/platform-operator-manager-rolebinding configured
clusterrolebinding.rbac.authorization.k8s.io/platform-operator-proxy-rolebinding configured
service/platform-operator-controller-manager-metrics-service configured
service/platform-operator-webhook-service configured
deployment.apps/platform-operator-controller-manager configured
certificate.cert-manager.io/platform-operator-serving-cert configured
issuer.cert-manager.io/platform-operator-selfsigned-issuer configured
validatingwebhookconfiguration.admissionregistration.k8s.io/platform-operator-validating-webhook-configuration configured
```

This command use `kustomize` to build the manifests. Once ready the manifests are applied to the cluster and the operator starts.
