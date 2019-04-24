# DORA controller
This controller collects DORA metrics:

* lead time (commit to deployment)

* deploy frequency

* mean time to recovery

* change fail percentage

To gather metrics for your deployment, add the following annotation(s):

* `dora/team` (e.g. `frontend`, `ops`) - required
* `dora/commitTimestamp` (Unix time, e.g. 1555126199)

Use the Helm chart for in-cluster deployment. For out-of cluster use, specify the path to your Kubernetes config file or export KUBECONFIG:

```
$ ./dora-controller --kubeconfig=${HOME}/.kube/config
```
