# cli

## Example Files

To run the `generate` command you need two files.

A configuration and parameters.

### Configuration

```apiVersion: opdef.apps.onepanel.io/v1alpha1
kind: OpDef
spec:
  manifestsRepo: /Users/andrey/projects/onepanel/manifests # This needs to be a path on your computer to the manifests repository
  params: params.env # This needs to be a path to a file containing environment variables you want to use.
  components:
   - istio
   - argo
   - storage
  overlays:
   - storage/overlays/gcp
   - common/cert-manager/overlays/aws
```
   
   
### params.env (can be renamed)

An environment variables file. 

```email=admin+test@onepanel.io
commonName=*.test-cluster-5.onepanel.io

aws_region=us-west-2
aws_access_key=aws_access_key
```
