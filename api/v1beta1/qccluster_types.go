/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows ReconcileQCCluster to clean up QingCloud resources associated with QCCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "qccluster.infrastructure.cluster.x-k8s.io"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QCClusterSpec defines the desired state of QCCluster
type QCClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The QingCloud Region the cluster lives in.
	Zone string `json:"zone"`
	// NetworkSpec encapsulates all things related to GCP network.
	// +optional
	Network QCNetwork `json:"network"`
	// ControlPlaneEndpoint represents the endpoint used to communicate with the
	// control plane. If ControlPlaneDNS is unset, the QC load-balancer IP
	// of the Kubernetes API Server is used.
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`
	// StorageClass management storageClass in cluster
	// if set to true, it will auto install openebs in cluster
	StorageClass Enable `json:"storageClass,omitempty"`
	// Ingress management Ingress controller in cluster
	// if set to true, it will auto install nginx-ingress in cluster
	Ingress Enable `json:"ingress,omitempty"`
	// Repository management image repository in cluster
	// if set to true, it will auto install harbor in cluster
	Repository Enable `json:"repository,omitempty"`
	// KubeSphere is a distributed operating system for cloud-native application management
	// if set to true, it will auto install harbor in cluster
	KubeSphere Enable `json:"kubesphere,omitempty"`
	// ClusterAutoScale manage the cluster nodes
	// if enabled, it will auto scale the cluster nodes.
	// +optional
	ClusterAutoScale ClusterAutoScale `json:"clusterAutoScale,omitempty"`
}

// QCClusterStatus defines the observed state of QCCluster
type QCClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Ready denotes that the cluster (infrastructure) is ready.
	// +optional
	Ready bool `json:"ready"`
	// Network encapsulates all things related to DigitalOcean network.
	// +optional
	Network QCNetworkResources `json:"network,omitempty"`
	// CurrentReplicas is current number of replicas of nodes managed by this clusterAutoScaler,
	// as last seen by the clusterAutoScaler.
	// +optional
	CurrentReplicas int `json:"currentReplicas,omitempty"`

	// DesiredReplicas is the desired number of replicas of nodes managed by this clusterAutoScaler,
	// as last calculated by the clusterAutoScaler.
	DesiredReplicas int `json:"desiredReplicas,omitempty"`
}

type Enable struct {
	Enabled bool `json:"enabled,omitempty"`
}

type ClusterAutoScale struct {
	// Enabled clusterAutoScale, if it's not specified, default false.
	Enabled bool `json:"enabled,omitempty"`

	// MinReplicas is the lower limit for the number of replicas to which the clusterAutoScale
	// can scale down.  It defaults to 1 node.  minReplicas is allowed to be 0.  Scaling is
	// active as long as at least one metric value is available.
	// +optional
	MinReplicas int32 `json:"minReplicas,omitempty"`
	// MaxReplicas is the upper limit for the number of replicas to which the clusterAutoScale can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas,omitempty"`
	// target average CPU utilization (represented as a percentage of requested CPU) over all the nodes;
	// if not specified the default autoscaling policy will be used.
	// +optional
	TargetCPUUtilizationPercentage int32 `json:"averageCPUUtilization,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:path=qcclusters,scope=Namespaced,categories=cluster-api
//+kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this QCCluster belongs"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Cluster infrastructure is ready for QingCloud instances"
//+kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.ControlPlaneEndpoint",description="API Endpoint",priority=1

// QCCluster is the Schema for the qcclusters API
type QCCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QCClusterSpec   `json:"spec,omitempty"`
	Status QCClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QCClusterList contains a list of QCCluster
type QCClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QCCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QCCluster{}, &QCClusterList{})
}
