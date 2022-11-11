# Kubernetes Cluster API Provider QingCloud

------
Kubernetes-native declarative infrastructure for QingCloud.

## What is the Cluster API Provider QingCloud
The [Cluster API][cluster_api] brings
declarative, Kubernetes-style APIs to cluster creation, configuration and
management.

The API itself is shared across multiple cloud providers allowing for true QingCloud
hybrid deployments of Kubernetes. It is built atop the lessons learned from
previous cluster managers such as [kops][kops] and
[kubicorn][kubicorn].

## Launching a Kubernetes cluster on QingCloud
### Building OS image
Before using cluster-api-provider-qingcloud, you need to [build a system image](./docs/os-img.md) containing k8s components on QingCloud.
### Initialize the management cluster
#### Install Cluster-API
```shell
curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.1.3/clusterctl-linux-amd64 -o clusterctl
chmod +x ./clusterctl
sudo mv ./clusterctl /usr/local/bin/clusterctl
```
#### Init Cluster-API controllers
```shell
clusterctl init
```
#### Deploy cluster-api provider qingcloud 
```shell
kubectl apply -f https://raw.githubusercontent.com/kubesphere/cluster-api-provider-qingcloud/main/manifests.yaml
```
#### Create api key secret for QingCloud
```shell
kubectl create secret generic -n capqc-system capqc-manager-bootstrap-credentials --from-literal=qy_access_key_id=${ACCESS_KEY_ID} --from-literal=qy_secret_access_key=${SECRET_ACCESS_KEY}
```
### Creating a workload cluster
#### Create cluster resource
```shell
cat <<EOF | kubectl apply -f -
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: qc-test
  namespace: default
spec:
  clusterNetwork:
    pods:
      cidrBlocks:
      - 172.16.0.0/16
  controlPlaneRef:
    apiVersion: controlplane.cluster.x-k8s.io/v1beta1
    kind: KubeadmControlPlane
    name: qc-test-control-plane
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: QCCluster
    name: qc-test

---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: QCCluster
metadata:
  name: qc-test
  namespace: default
spec:
  zone: ap2a
  ## Creating a cluster in an existing vpc needs to be configured as follows
  ## EIP and Port for ControlPlaneEndpoint
  #  controlPlaneEndpoint:
  #    host: 139.198.120.22
  #    port: 6440
  #  network:
  #    vpc:
  #      resourceID: rtr-b7kvpnwv
  ## ipNetwork required for vxnet to join vpc.
  ## This network must be unused in your vpc.
  #    vxnets:
  #    - ipNetwork: 192.168.100.0/24

EOF
```
#### Create control plane resources 
```shell
cat <<EOF | kubectl apply -f -
---
apiVersion: controlplane.cluster.x-k8s.io/v1beta1
kind: KubeadmControlPlane
metadata:
  name: qc-test-control-plane
  namespace: default
spec:
  kubeadmConfigSpec:
    initConfiguration:
      nodeRegistration:
        criSocket: unix:///run/containerd/containerd.sock
        kubeletExtraArgs:
          cloud-provider: external
          provider-id: qingcloud://${instance_id}
        name: ${instance_name}
    joinConfiguration:
      nodeRegistration:
        criSocket: unix:///run/containerd/containerd.sock
        kubeletExtraArgs:
          cloud-provider: external
          provider-id: qingcloud://${instance_id}
        name: ${instance_name}

  machineTemplate:
    infrastructureRef:
      apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
      kind: QCMachineTemplate
      name: qc-test-control-plane
  replicas: 1
  version: v1.23.6

---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: QCMachineTemplate
metadata:
  name: qc-test-control-plane
  namespace: default
spec:
  template:
    spec:
      instanceType: s1.large.r1
      imageID: img-3tuakulu
      # It need to be replaced with your own sshkey.
      sshKey: kp-jcne0ht0     
      
EOF
```

#### Create workers resources
```shell
cat <<EOF | kubectl apply -f -
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
metadata:
  name: qc-test-md-0
  namespace: default
spec:
  clusterName: qc-test
  replicas: 1
  selector:
    matchLabels: null
  template:
    spec:
      bootstrap:
        configRef:
          apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
          kind: KubeadmConfigTemplate
          name: qc-test-md-0
      clusterName: qc-test
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
        kind: QCMachineTemplate
        name: qc-test-md-0
      version: v1.23.6
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: QCMachineTemplate
metadata:
  name: qc-test-md-0
  namespace: default
spec:
  template:
    spec:
      instanceType: s1.large.r1
      imageID: img-3tuakulu
      # It need to be replaced with your own sshkey.
      sshKey: kp-jcne0ht0
---
apiVersion: bootstrap.cluster.x-k8s.io/v1beta1
kind: KubeadmConfigTemplate
metadata:
  name: qc-test-md-0
  namespace: default
spec:
  template:
    spec:
      joinConfiguration:
        nodeRegistration:
          criSocket: unix:///run/containerd/containerd.sock
          kubeletExtraArgs:
            cloud-provider: external
            provider-id: qingcloud://${instance_id}
          name: ${instance_name}
EOF
```
#### Scale cluster
```shell
kubectl scale machinedeployment qc-test-md-0 --replicas=3
```

<!-- References -->

[prow]: https://go.k8s.io/bot-commands
[issue]: https://github.com/kubernetes-sigs/cluster-api-provider-digitalocean/issues
[new_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-digitalocean/issues/new
[good_first_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-digitalocean/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22
[cluster_api]: https://github.com/kubernetes-sigs/cluster-api
[kops]: https://github.com/kubernetes/kops
[kubicorn]: http://kubicorn.io/
[tilt]: https://tilt.dev
[cluster_api_tilt]: https://master.cluster-api.sigs.k8s.io/developer/tilt.html