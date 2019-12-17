# platform-operator

Platform Operator is a kubernetes operator designed to be run in a multi-tenanted environment.

1. Installing

```bash
kubectl apply -f https://raw.githubusercontent.com/flanksource/platform-operator/master/deploy/platform-operator.yaml
```

### Auto-Delete

The operator can automatically cleanup namespaces after a certain expiry period by labeleling the namespace with `auto-delete` e.g.

```bash
kubectl label ns/pr-123 "auto-delete=24h"
```
