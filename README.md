# Platform Operator

Platform Operator is Kubernetes operator designed to be run in a multi-tenanted environment.

Current features:

* Auto-Delete: cleanup namespaces after a certain expiry period by labeleling the namespace with `auto-delete`.
* ClusterResourceQuota: allows quotas to be enforced across the entire cluster.

## Install

1. Generate the YAML manifests containing all the resources (CRDs, namespaces, Deployment, etc...)

```shell
make generate
```

This command will create the file [manifests.yaml](config/deploy/manifests.yaml). Don't make manual changes to this file.

2. Deploy the generated configuration in the cluster:

```shell
make deploy
kubectl apply -f config/deploy/manifests.yaml
namespace/flanksource-system created
customresourcedefinition.apiextensions.k8s.io/clusterresourcequotas.platform.flanksource.com created
validatingwebhookconfiguration.admissionregistration.k8s.io/flanksource-validating-webhook-configuration created
role.rbac.authorization.k8s.io/flanksource-leader-election created
clusterrole.rbac.authorization.k8s.io/flanksource-clusterresourcequota-editor created
clusterrole.rbac.authorization.k8s.io/flanksource-clusterresourcequota-viewer created
clusterrole.rbac.authorization.k8s.io/flanksource-manager created
rolebinding.rbac.authorization.k8s.io/flanksource-leader-election created
clusterrolebinding.rbac.authorization.k8s.io/flanksource-manager created
service/flanksource-webhook-service created
deployment.apps/flanksource-controller-manager created
certificate.cert-manager.io/flanksource-serving-cert configured
issuer.cert-manager.io/flanksource-selfsigned-issuer configured
```

This command use `kustomize` to build the manifests. Once ready the manifests are applied to the cluster and the operator starts.
