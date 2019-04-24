# DORA controller
This controller collects DORA metrics.

Add annotation `dora/team` with an appropriate value (e.g. `frontend`, `ops`) to your deployment to monitor deployment success or otherwise.

## Run out of cluster
```
$ go build
$ ./dora-controller --kubeconfig=${HOME}/.kube/config
```
