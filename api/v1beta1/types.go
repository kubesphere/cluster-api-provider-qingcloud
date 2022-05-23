package v1beta1

import (
	"strings"
)

type QCResourceID *string

// QCNetwork encapsulates QingCloud networking configuration.
type QCNetwork struct {
	// EIP configuration.
	// +optional
	EIP QCEIP `json:"eip,omitempty"`
	// VxNets configuration.
	VxNets []QCVxNet `json:"vxnets,omitempty"`
	// VPC defines the VPC configuration.
	// +optional
	VPC QCVPC `json:"vpc,omitempty"`
	// SecurityGroup defines the SecurityGroup configuration.
	// +optional
	SecurityGroup SecurityGroup `json:"securityGroup,omitempty"`
	// Configures an API Server loadbalancers
	// +optional
	APIServerLoadbalancer QCLoadBalancer `json:"apiServerLoadbalancer,omitempty"`
}

type QCEIP struct {
	// ResourceID defines the EIP ID to use. If omitted, a new EIP will be created.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
	// Bandwidth defines the EIP bandwidth to use. default(10M/s).
	// +optional
	Bandwidth int `json:"bandwidth,omitempty"`
	// BillingMode defines the EIP BillingMode to use. [bandwidth / traffic] default("traffic").
	// +optional
	BillingMode string `json:"billingMode,omitempty"`
}
type QCVxNet struct {
	// ResourceID defines the VxNet ID to use. If omitted, a new VxNet will be created.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
	IPNetwork  string `json:"ipNetwork,omitempty"`
}

type SecurityGroup struct {
	// The QingCloud load balancer ID. If omitted, a new load balancer will be created.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
}

type QCVPC struct {
	// ResourceID defines the Router ID to use. If omitted, a new VPC router will be created.
	// +optional
	ReclaimPolicy ReclaimPolicy `json:"reclaimPolicy,omitempty"`
	ResourceID    string        `json:"resourceID,omitempty"`
}

// ReclaimPolicy describes a policy for end-of-life maintenance of vpc.
// +enum
type ReclaimPolicy string

const (
	// ReclaimDelete means the vpc will be deleted from Kubernetes on release from its claim.
	ReclaimDelete ReclaimPolicy = "Delete"
	// ReclaimRetain means the vpc will be left in its current phase (Released) for manual reclamation by the administrator.
	// The default policy is Retain.
	ReclaimRetain ReclaimPolicy = "Retain"
)

type QCLoadBalancer struct {
	// The QingCloud load balancer ID. If omitted, a new load balancer will be created.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
}

// QCNetworkResources encapsulates QingCloud networking resources.
type QCNetworkResources struct {
	// APIServerLoadbalancersRef is the id of apiserver loadbalancers.
	// +optional
	APIServerLoadbalancersRef QCResourceReference `json:"apiServerLoadbalancersRef,omitempty"`
	// APIServerLoadbalancersListener is the id of apiserver loadbalancers listener.
	// +optional
	APIServerLoadbalancersListenerRef QCResourceReference `json:"APIServerLoadbalancersListenerRef,omitempty"`
	// EIPRef is the id of eip.
	// +optional
	EIPRef QCResourceReference `json:"eipRef,omitempty"`
	// RouterRef is the id of router.
	// +optional
	RouterRef QCResourceReference `json:"routerRef,omitempty"`
	// VxNetRef is the id of VxNet.
	// +optional
	VxNetsRef []VxNetRef `json:"vxnetsRef,omitempty"`
	// SecurityGroupRef is the id of SecurityGroup.
	// +optional
	SecurityGroupRef QCResourceReference `json:"securityGroupRef,omitempty"`
}

type VxNetRef struct {
	// +optional
	IPNetwork string `json:"ipNetwork,omitempty"`
	// +optional
	ResourceRef QCResourceReference `json:"resourceRef,omitempty"`
}

// QCResourceReference is a reference to a QingCloud resource.
type QCResourceReference struct {
	// ID of QingCloud resource
	// +optional
	ResourceID string `json:"resourceId,omitempty"`
	// Status of QingCloud resource
	// +optional
	ResourceStatus QCResourceStatus `json:"resourceStatus,omitempty"`
}

// QCMachineTemplateResource describes the data needed to create an QCMachine from a template.
type QCMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec QCMachineSpec `json:"spec"`
}

// QCResourceStatus describes the status of a QingCloud resource.
type QCResourceStatus string

var (
	// QCResourceStatusPending is the string representing an QingCloud resource in a pending state.
	QCResourceStatusPending = QCResourceStatus("pending")

	// QCResourceStatusActive is the string representing an QingCloud resource in a active state.
	QCResourceStatusActive = QCResourceStatus("active")

	// QCResourceStatusRunning is the string representing an QingCloud resource in a running state.
	QCResourceStatusRunning = QCResourceStatus("running")

	// QCResourceStatusStopped is the string representing an QingCloud resource in a stopped state.
	// that has been stopped and can be restarted.
	QCResourceStatusStopped = QCResourceStatus("stopped")

	// QCResourceStatusClear is the string representing an QingCloud resource in a clear state.
	QCResourceStatusClear = QCResourceStatus("clear")

	// QCResourceStatusDeleted is the string representing an QingCloud resource in a deleted state.
	QCResourceStatusDeleted = QCResourceStatus("deleted")

	// QCResourceStatusDeleteing is the string representing an QingCloud resource in a deleteing state.
	QCResourceStatusDeleteing = QCResourceStatus("deleteing")

	// QCResourceStatusCeased is the string representing an QingCloud resource in a ceased state.
	QCResourceStatusCeased = QCResourceStatus("ceased")
)

type InstanceSize struct {
	// RootDeviceSize is the size of the root volume in GB.
	// Defaults to 40.
	// +optional
	RootDevice int64 `json:"rootDevice,omitempty"`
	CPU        int64 `json:"cpu,omitempty"`
	Memory     int64 `json:"memory,omitempty"`
}

// QCSafeName returns QingCloud safe name with replacing '.' and '/' to '-'
// since QingCloud doesn't support naming with those character.
func QCSafeName(name string) string {
	r := strings.NewReplacer(".", "-", "/", "-")
	return r.Replace(name)
}
