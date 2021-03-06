# Adding Latest CRO to RHMI
Once the CRO release has been cut you will need a PR to the integreatly-operator to
add the latest version of CRO

## Update the version of CRO in the Integreatly-operator

```bash
go get github.com/integr8ly/cloud-resource-operator
```
This should update the `go.mod` and `go.sum` file with the correct version from master

The integreatly-operator has a make command for fixing vendor
```cassandraql
make vendor/fix
```
It should also add the latest version of CRO to the `vendor/` directory and update
`vendor/modules.txt`

## Update the CSV in CRO manifest for the Integreatly-operator
We typically keep n-1 versions of manifests due to a config map size limitation, 
- If there were multiple releases of CRO we keep the version that was in use at the time of the last RHMI release **(n-1)** 
- Add latest version of CRO **(n)**

Remove the oldest version directory from 
- https://github.com/integr8ly/integreatly-operator/tree/master/manifests/integreatly-cloud-resources

Remove the `replaces` line from the remaining versions csv
e.g. 
- https://github.com/integr8ly/integreatly-operator/blob/4cade544489acd8a9107f687d92a8d893a3db257/manifests/integreatly-cloud-resources/0.15.2/cloud-resources.v0.15.2.clusterserviceversion.yaml#L344

Copy into [this](https://github.com/integr8ly/integreatly-operator/tree/master/manifests/integreatly-cloud-resources) directory the latest version from CRO 
- `./deploy/olm-catalog/cloud-resources/<latest-version>`

Update the version in the `cloud-resource-operator.package.yaml` to the `<latest-version>`
e.g. 
- https://github.com/integr8ly/integreatly-operator/blob/v2.2.0/manifests/integreatly-cloud-resources/cloud-resource-operator.package.yaml#L2

Update the CRO version in rhmi-types file
e.g. 
- https://github.com/integr8ly/integreatly-operator/blob/v2.2.0/pkg/apis/integreatly/v1alpha1/rhmi_types.go#L64
- https://github.com/integr8ly/integreatly-operator/blob/v2.2.0/pkg/apis/integreatly/v1alpha1/rhmi_types.go#L91

## Verification
- Install RHMI on byoc cluster
 
- Upgrade rhmi on byoc cluster, See the upgrade [sop](https://github.com/RHCloudServices/integreatly-help/blob/11eef40ff3f34cea64810cfbe93a4c2b280b7d07/sops/2.x/upgrade/upgrade_cluster_rhmi_SOP.md)

