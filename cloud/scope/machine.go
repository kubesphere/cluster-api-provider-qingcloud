package scope

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/pointer"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ProviderIDPrefix is the qingcloud provider id prefix.
	ProviderIDPrefix = "qingcloud://"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	QCClients
	Client    client.Client
	Logger    logr.Logger
	Cluster   *clusterv1beta1.Cluster
	Machine   *clusterv1beta1.Machine
	QCCluster *infrav1beta1.QCCluster
	QCMachine *infrav1beta1.QCMachine
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster   *clusterv1beta1.Cluster
	Machine   *clusterv1beta1.Machine
	QCCluster *infrav1beta1.QCCluster
	QCMachine *infrav1beta1.QCMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration
// both QCClusterReconciler and QCMachineReconciler.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("Client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("Machine is required when creating a MachineScope")
	}
	if params.Cluster == nil {
		return nil, errors.New("Cluster is required when creating a MachineScope")
	}
	if params.QCCluster == nil {
		return nil, errors.New("QCCluster is required when creating a MachineScope")
	}
	if params.QCMachine == nil {
		return nil, errors.New("QCMachine is required when creating a MachineScope")
	}

	if params.Logger == (logr.Logger{}) {
		params.Logger = klogr.New()
	}
	helper, err := patch.NewHelper(params.QCMachine, params.Client)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init patch helper")
	}
	return &MachineScope{
		Logger:      params.Logger,
		client:      params.Client,
		patchHelper: helper,
		Cluster:     params.Cluster,
		Machine:     params.Machine,
		QCCluster:   params.QCCluster,
		QCMachine:   params.QCMachine,
	}, nil
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close() error {
	return m.patchHelper.Patch(context.TODO(), m.QCMachine)
}

// Name returns the QCMachine name.
func (m *MachineScope) Name() string {
	return m.QCMachine.Name
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.QCMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return infrav1beta1.APIServerRoleTagValue
	}
	return infrav1beta1.NodeRoleTagValue
}

// GetInstanceStatus returns the QCMachine instance status.
func (m *MachineScope) GetInstanceStatus() *infrav1beta1.QCResourceStatus {
	return m.QCMachine.Status.InstanceStatus
}

// SetInstanceStatus sets the QCMachine droplet id.
func (m *MachineScope) SetInstanceStatus(v infrav1beta1.QCResourceStatus) {
	m.QCMachine.Status.InstanceStatus = &v
}

// SetProviderID sets the GCPMachine providerID in spec.
func (m *MachineScope) SetProviderID(instanceID string) {
	providerID := fmt.Sprintf("%s%s", ProviderIDPrefix, instanceID)
	m.QCMachine.Spec.ProviderID = pointer.StringPtr(providerID)
}

// GetInstanceID returns the QCMachine instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetInstanceID() *string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return nil
	}

	return pointer.StringPtr(parsed.ID())
}

// GetProviderID returns the QCMachine providerID from the spec.
func (m *MachineScope) GetProviderID() string {
	if m.QCMachine.Spec.ProviderID != nil {
		return *m.QCMachine.Spec.ProviderID
	}
	return ""
}

// SetReady sets the QCMachine Ready Status.
func (m *MachineScope) SetReady() {
	m.QCMachine.Status.Ready = true
}

// SetFailureReason sets the QCMachine status failure reason.
func (m *MachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.QCMachine.Status.FailureReason = &v
}

// SetFailureMessage sets the QCMachine status error message.
func (m *MachineScope) SetFailureMessage(v error) {
	m.QCMachine.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData() ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(context.TODO(), key, secret); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for DOMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}
