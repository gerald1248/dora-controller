# DORA controller
This controller collects four DORA metrics:

* lead time (commit to deployment)

* deploy frequency

* mean time to recovery

* change fail percentage

To gather metrics for your deployment, add the following annotation(s):

* `dora/team` (e.g. `frontend`, `ops`) - required

* `dora/commitTimestamp` (Unix time, e.g. `"1555126199"`) - optional; required only for lead time measurement

Note that the numerical commit timestamp must be quoted for the deployment to validate. (Annotation values are strings.)

## Maintaining state
The controller does not retain state beyond a simple datastructure mapping teams to deployment names with certain properties of the most recent deployment.

Single-line JSON log messages are written to STDOUT. Aggregation and data analysis is pushed out to central log management (be that EFK, Splunk or another kind of tool entirely).

**Lead time** is retrieved by filtering for flag `DORA_SUCCESSFUL_DEPLOYMENT` and the `leadTimeSeconds` attribute. (Only available when deployments are annotated as indicated above.)

**Deployment frequency** can be measured by counting log entries with flag `DORA_SUCCESSFUL_DEPLOYMENT` (and, if desired, `DORA_FAILED_DEPLOYMENT`).

To compute the **mean time to recovery** over a period of time, filter for `DORA_RECOVERY` and the `recoverySeconds` attribute.

The **change fail percentage** is `DORA_FAILED_DEPLOYMENT` divided by `DORA_SUCCESSFUL_DEPLOYMENT || 1` times one hundred.

## Known limitations

No attempt is made to distinguish between regular deployment, patches, hotfixes, and so on. Any deployment with a changed Docker image or tag is treated as a new code release. To measure lead time, the build/CI system has to record the timestamp of the final git commit in annotation form.

## Deployment

Use or adapt the Helm chart for in-cluster deployment. `helm template .` will write out suitable manifests. For out-of cluster use, specify the path to your Kubernetes config file or export KUBECONFIG:

```
$ ./dora-controller --kubeconfig=${HOME}/.kube/config
```
