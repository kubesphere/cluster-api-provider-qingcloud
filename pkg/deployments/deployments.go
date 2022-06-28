package deployments

import (
	_ "embed"
)

//go:embed metrics.yaml
var MetricsResources []byte

//go:embed calico.yaml
var CalicoResources []byte

//go:embed qingcloudcsi.yaml
var QingCloudCSIReources []byte

//go:embed kubesphere.yaml
var KubeSphereResources []byte

//go:embed nginx-ingress.yaml
var IngressResources []byte

//go:embed openebs.yaml
var OpenebsResources []byte

var QingCloudCSITemp = `
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: csi-qingcloud
    owner: yunify
    ver: 1.3.2
  name: csi-qingcloud
  namespace: kube-system
data:
  config.yaml: |-
    qy_access_key_id: {{.QCAccessKeyID}}
    qy_secret_access_key: {{.QCAccessKey}}
    zone: {{.Zone}}
    host: api.qingcloud.com
    port: 443
    protocol: https
    uri: /iaas
    connection_retries: 3
    connection_timeout: 30
`
