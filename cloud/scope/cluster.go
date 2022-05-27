package scope

import (
	"context"
	"github.com/go-logr/logr"
	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/pkg/errors"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	QCClients
	Client    client.Client
	Logger    logr.Logger
	Cluster   *clusterv1.Cluster
	QCCluster *infrav1beta1.QCCluster
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	QCClients
	Cluster   *clusterv1.Cluster
	QCCluster *infrav1beta1.QCCluster
}

// NewClusterScope creates a new ClusterScope from the supplied parameters.
// This is meant to be called for each reconcile iteration only on QCClusterReconciler.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("Cluster is required when creating a ClusterScope")
	}
	if params.QCCluster == nil {
		return nil, errors.New("QCCluster is required when creating a ClusterScope")
	}
	if params.Logger == (logr.Logger{}) {
		params.Logger = klogr.New()
	}
	helper, err := patch.NewHelper(params.QCCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &ClusterScope{
		Logger:      params.Logger,
		client:      params.Client,
		patchHelper: helper,
		QCClients:   params.QCClients,
		Cluster:     params.Cluster,
		QCCluster:   params.QCCluster,
	}, nil
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.QCCluster)
}

// SetControlPlaneEndpoint sets the QCCluster status APIEndpoints.
func (s *ClusterScope) SetControlPlaneEndpoint(apiEndpoint clusterv1.APIEndpoint) {
	s.QCCluster.Spec.ControlPlaneEndpoint = apiEndpoint
}

// SetReady sets the DOCluster Ready Status.
func (s *ClusterScope) SetReady() {
	s.QCCluster.Status.Ready = true
}

// Name returns the cluster name.
func (s *ClusterScope) Name() string {
	return s.Cluster.GetName()
}

// UID returns the cluster UID.
func (s *ClusterScope) UID() string {
	return string(s.Cluster.UID)
}

// APIServerLoadbalancers get the QCCluster Spec Network APIServerLoadbalancers.
func (s *ClusterScope) APIServerLoadbalancerSpec() *infrav1beta1.QCLoadBalancer {
	return &s.QCCluster.Spec.Network.APIServerLoadbalancer
}

// APIServerLoadbalancersRef get the QCCluster status Network APIServerLoadbalancersRef.
func (s *ClusterScope) APIServerLoadbalancerRef() *infrav1beta1.QCResourceReference {
	return &s.QCCluster.Status.Network.APIServerLoadbalancersRef
}

// APIServerLoadbalancersListenerRef get the QCCluster status Network APIServerLoadbalancersListenerRef.
func (s *ClusterScope) APIServerLoadbalancerListenerRef() *infrav1beta1.QCResourceReference {
	return &s.QCCluster.Status.Network.APIServerLoadbalancersListenerRef
}

// EIP get the QCCluster Spec Network EIP.
func (s *ClusterScope) EIPSpec() *infrav1beta1.QCEIP {
	return &s.QCCluster.Spec.Network.EIP
}

// EIPRef get the QCCluster status Network EIPRef.
func (s *ClusterScope) EIPRef() *infrav1beta1.QCResourceReference {
	return &s.QCCluster.Status.Network.EIPRef
}

// VxNets get the QCCluster Spec Network VxNets.
func (s *ClusterScope) VxNetsSpec() []infrav1beta1.QCVxNet {
	return s.QCCluster.Spec.Network.VxNets
}

// Zone get the QCCluster Spec zone.
func (s *ClusterScope) Zone() string {
	return s.QCCluster.Spec.Zone
}

// VxNetRef get the QCCluster status Network VxNetRef.
func (s *ClusterScope) VxNetRef() []infrav1beta1.VxNetRef {
	return s.QCCluster.Status.Network.VxNetsRef
}

// Router get the QCCluster Spec Network Router.
func (s *ClusterScope) RouterSpec() *infrav1beta1.QCVPC {
	return &s.QCCluster.Spec.Network.VPC
}

// RouterRef get the QCCluster status Network RouterRef.
func (s *ClusterScope) RouterRef() *infrav1beta1.QCResourceReference {
	return &s.QCCluster.Status.Network.RouterRef
}

// SecurityGroupRef get the QCCluster status Network SecurityGroupRef.
func (s *ClusterScope) SecurityGroupRef() *infrav1beta1.QCResourceReference {
	return &s.QCCluster.Status.Network.SecurityGroupRef
}

// SecurityGroup get the QCCluster Network SecurityGroup id.
func (s *ClusterScope) SecurityGroup() *infrav1beta1.SecurityGroup {
	return &s.QCCluster.Spec.Network.SecurityGroup
}

// GetVPCReclaimPolicy get the VPC Reclaim Policy.
func (s *ClusterScope) GetVPCReclaimPolicy() infrav1beta1.ReclaimPolicy {
	return s.QCCluster.Spec.Network.VPC.ReclaimPolicy
}
