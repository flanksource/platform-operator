# Platform Operator

Platform Operator is Kubernetes operator designed to be run in a multi-tenanted environment.

### Namespaced Tolerations

Applies tolerations to all pods in a namespace, based on annotations on the namespace

e.g. using`--enable-pod-mutations=true --namespace-tolerations-prefix=tolerations`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: dedicate-to-node-group-b
  annotations:
    tolerations/node-group: b
```

Will then result in all pods created in that namespace receiving a toleration of:

```yaml
apiVersion: v1
kind: Pod
spec:
   tolerations:
     key: node-group
     value: b
     effect: NoSchedule
```

### Namespace Annotation Defaults

e.g. with `--enable-pod-mutations=true --annotations=co.elastic`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: dedicate-to-node-group-b
  annotations:
    co.elastic.logs/enabled: true
```

Will then result in all pods created in that namespace defaulting to:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    co.elastic.logs/enabled: true
```

### Registry Defaults

e.g. with `--enable-pod-mutations=true --default-registry-prefix==registry.corp`

When creating a pod with a `busybox:latest`  such as:

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - image: busybox:latest
```

It will get mutated to:

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - image: registry.corp/busybox:latest
```

To prevent some images from being prefixed use `--registry-whitelist` e.g.  `--registry-whitelist=k8s.gcr.io`

Add a default image pull secret to all pods using `--default-image-pull-secret`

### Auto Delete

- `--cleanup=true` - Delete resources with `auto-delete` annotations specified in duration from creation
  - `--cleanup-interval` - Interval to check for resources to cleanup

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: pr-workflow-123
  annotations:
     auto-delete: 24h # delete this namespace 24h after creation
```

### Cluster Resource Quotas

- `--enable-cluster-resource-quota` - Allow resource quotas to be defined at cluster level

```yaml
apiVersion: platform.flanksource.com/v1
kind: ClusterResourceQuota
metadata:
  name: dynamic-pr-compute-resources
spec:
  matchLabels:
    owner: dynamic-pr
  hard:
    requests.cpu: "1"
    requests.memory: 1Gi
    limits.cpu: "1"
    limits.memory: 1Gi
    pods: "10"
    services.loadbalancers: "0"
    services.nodeports: "0"

```



### Ingress SSO

Depends on karina ingress as is normally deployed only via karina using:

`karina.yml`

```yaml
domain: ACMP.corp
ldap:
	....
dex:
  version: v2.27.0
oauth2Proxy:
  version: v6.1.1
platformOperator:
  version: v0.6.0
```

- `--enable-ingress-sso` enable ingress SSO using `platform.flanksource.com/restrict-to-groups` annotations
  - `--oauth2-proxy-service-name`
  - `--oauth2-proxy-service-namespace`
  - `--domain`

>  See https://karina.docs.flanksource.com/admin-guide/ingress/ for more details on how to configure the ingress, before using the platform-operator.

Once installed ingresses can be restricted using:

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: podinfo-ing
  namespace: default
  annotations:
    kubernetes.io/tls-acme: "true"
    platform.flanksource.com/restrict-to-groups: ADMINS
```



| Annotation                                             | Description                                                  |
| ------------------------------------------------------ | ------------------------------------------------------------ |
| `platform.flanksource.com/restrict-to-groups`          | A semi-colon delimited list of LDAP groups to restrict an ingress to |
| `platform.flanksource.com/extra-configuration-snippet` | Any additional nginx snippets to apply to the location       |
| `platform.flanksource.com/pass-auth-headers`           | Specify `true` to pass authentication headers all the way through to the ingress upstream |
