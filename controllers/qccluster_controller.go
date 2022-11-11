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

package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/services/networking"
	qcerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors/qingcloud"
	"github.com/pkg/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// QCClusterReconciler reconciles a QCCluster object
type QCClusterReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
	WatchFilterValue string
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the QCCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *QCClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())

	qcCluster := &infrav1beta1.QCCluster{}
	if err := r.Get(ctx, req.NamespacedName, qcCluster); err != nil {
		if kubeerrors.IsNotFound(err) {
			logger.Info("QCCluster is not found, skipping reconcile")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, qcCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	qcClients, err := scope.NewQCClients(qcCluster.Spec.Zone)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the cluster scope.
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		QCClients: *qcClients,
		Client:    r.Client,
		Logger:    logger,
		Cluster:   cluster,
		QCCluster: qcCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any changes.
	defer func() {
		if err = clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !qcCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	return r.reconcile(ctx, clusterScope)
}

func (r *QCClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling QCCluster")
	qccluster := clusterScope.QCCluster
	// If the DOCluster doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(qccluster, infrav1beta1.ClusterFinalizer)

	networkingsvc := networking.NewService(ctx, clusterScope)

	var describeVPCOutput *qcs.DescribeRoutersOutput

	vpc := clusterScope.RouterSpec()
	vpcRef := clusterScope.RouterRef()

	securityGroup := clusterScope.SecurityGroup()
	securityGroupRef := clusterScope.SecurityGroupRef()

	eipRef := clusterScope.EIPRef()
	eip := clusterScope.EIPSpec()

	var err error
	if vpc.ResourceID != "" {
		describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpc.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
		}

		if qcs.StringValue(describeVPCOutput.RouterSet[0].SecurityGroupID) != "" {
			securityGroup.ResourceID = qcs.StringValue(describeVPCOutput.RouterSet[0].SecurityGroupID)
		}

		if qcs.StringValue(describeVPCOutput.RouterSet[0].EIP.EIPID) != "" {
			eip.ResourceID = qcs.StringValue(describeVPCOutput.RouterSet[0].EIP.EIPID)
		}
	}

	// create SecurityGroup
	if securityGroup.ResourceID == "" && securityGroupRef.ResourceID == "" {
		var o infrav1beta1.QCResourceID
		o, err = networkingsvc.CreateSecurityGroup()
		if err != nil {
			clusterScope.Error(err, "create security group failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if o != nil {
			clusterScope.Info("security group created", "id", qcs.StringValue(o))
			rules := []*qcs.SecurityGroupRule{
				{
					Action:                qcs.String("accept"),
					Direction:             qcs.Int(0),
					Priority:              qcs.Int(1),
					Protocol:              qcs.String("tcp"),
					SecurityGroupID:       o,
					SecurityGroupRuleName: qcs.String(fmt.Sprintf("k8s-apiserver-%s", clusterScope.Name())),
					Val1:                  qcs.String("6443"),
				},
			}

			_, err = networkingsvc.AddSecurityGroupRules(o, rules)
			if err != nil {
				clusterScope.Error(err, "add security group rules failed")
				if !qcerrors.IsAlreadyExisted(err) {
					return reconcile.Result{}, err
				}
			}

			var s *qcs.DescribeSecurityGroupsOutput
			s, err = networkingsvc.GetSecurityGroup(o)
			if err != nil {
				return reconcile.Result{}, err
			}

			securityGroupRef.ResourceID = qcs.StringValue(o)
			securityGroup.ResourceID = qcs.StringValue(o)
			if qcs.IntValue(s.SecurityGroupSet[0].IsApplied) == 1 {
				securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
			} else {
				securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusPending
			}
		}
	} else if securityGroupRef.ResourceID != "" && securityGroup.ResourceID == "" {
		securityGroup.ResourceID = securityGroupRef.ResourceID
	} else if securityGroup.ResourceID != "" && securityGroupRef.ResourceID == "" {
		securityGroupResourceID := qcs.String(securityGroup.ResourceID)
		if qccluster.Spec.ControlPlaneEndpoint.Port == 0 {
			return reconcile.Result{}, errors.New("Value is not expected in spec.ControlPlaneEndpoint.Port. Specify the control plane port for the security group.")
		}
		rules := []*qcs.SecurityGroupRule{
			{
				Action:                qcs.String("accept"),
				Direction:             qcs.Int(0),
				Priority:              qcs.Int(1),
				Protocol:              qcs.String("tcp"),
				SecurityGroupID:       securityGroupResourceID,
				SecurityGroupRuleName: qcs.String(fmt.Sprintf("k8s-apiserver-%s", clusterScope.Name())),
				Val1:                  qcs.String(strconv.FormatInt(int64(qccluster.Spec.ControlPlaneEndpoint.Port), 10)),
			},
		}

		_, err = networkingsvc.AddSecurityGroupRules(securityGroupResourceID, rules)
		if err != nil {
			clusterScope.Error(err, "add security group rules failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}

		var s *qcs.DescribeSecurityGroupsOutput
		s, err = networkingsvc.GetSecurityGroup(securityGroupResourceID)
		if err != nil {
			return reconcile.Result{}, err
		}

		securityGroupRef.ResourceID = qcs.StringValue(securityGroupResourceID)
		if qcs.IntValue(s.SecurityGroupSet[0].IsApplied) == 1 {
			securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
		} else {
			securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusPending
		}
	}

	// wait for securityGroup ready
	s, err := networkingsvc.GetSecurityGroup(qcs.String(securityGroupRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	if qcs.IntValue(s.SecurityGroupSet[0].IsApplied) == 1 {
		securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
	} else {
		securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusPending
	}

	if securityGroupRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("security group not ready")
		return reconcile.Result{}, errors.Errorf("unexpected security group status: %v", securityGroupRef.ResourceStatus)
	}

	// create eip
	if eip.ResourceID == "" && eipRef.ResourceID == "" {
		if eip.BillingMode == "" {
			eip.BillingMode = networking.BillModeTraffic
		}
		if eip.Bandwidth == 0 {
			eip.Bandwidth = 100
		}
		var eipID infrav1beta1.QCResourceID
		eipID, err = networkingsvc.CreateEIP(eip.Bandwidth, eip.BillingMode)
		if err != nil {
			clusterScope.Error(err, "create eip failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if eipID != nil {
			clusterScope.Info("eip created", "id", qcs.StringValue(eipID))
			eip.ResourceID = qcs.StringValue(eipID)
			eipRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
		}
	} else if eipRef.ResourceID != "" && eip.ResourceID == "" {
		eip.ResourceID = eipRef.ResourceID
	} else {
		eipRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
	}

	// create VxNet
	vxnetsRef := clusterScope.VxNetRef()
	vxnet := clusterScope.VxNetsSpec()
	vxnetID := ""
	ipnetwork := "192.168.0.0/24"
	if len(vxnet) != 0 {
		vxnetID = vxnet[0].ResourceID
		if vxnet[0].IPNetwork != "" {
			ipnetwork = vxnet[0].IPNetwork
		}
	}
	vxnetRef := infrav1beta1.VxNetRef{
		IPNetwork: "",
		ResourceRef: infrav1beta1.QCResourceReference{
			ResourceID:     "",
			ResourceStatus: "",
		},
	}
	if len(vxnetsRef) != 0 {
		vxnetRef = vxnetsRef[0]
	}
	if vxnetID == "" && vxnetRef.ResourceRef.ResourceID == "" {
		var vID infrav1beta1.QCResourceID
		vID, err = networkingsvc.CreateVxNet()
		if err != nil {
			clusterScope.Error(err, "create vxnet failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if vID != nil {
			clusterScope.Info("vxnet created", "id", qcs.StringValue(vID))
			vxnetID = qcs.StringValue(vID)
			qccluster.Spec.Network.VxNets = []infrav1beta1.QCVxNet{{ResourceID: vxnetID, IPNetwork: ipnetwork}}
			qccluster.Status.Network.VxNetsRef = []infrav1beta1.VxNetRef{{IPNetwork: ipnetwork, ResourceRef: infrav1beta1.QCResourceReference{ResourceID: vxnetID, ResourceStatus: infrav1beta1.QCResourceStatusActive}}}
		}
	} else if vxnetRef.ResourceRef.ResourceID != "" || vxnetID == "" {
		qccluster.Spec.Network.VxNets = []infrav1beta1.QCVxNet{{ResourceID: vxnetRef.ResourceRef.ResourceID, IPNetwork: vxnetRef.IPNetwork}}
		vxnetID = vxnetRef.ResourceRef.ResourceID
	} else {
		qccluster.Status.Network.VxNetsRef = []infrav1beta1.VxNetRef{{IPNetwork: ipnetwork, ResourceRef: infrav1beta1.QCResourceReference{ResourceID: vxnetID, ResourceStatus: infrav1beta1.QCResourceStatusActive}}}
	}

	// create VPC
	if vpc.ResourceID == "" && vpcRef.ResourceID == "" {
		var o infrav1beta1.QCResourceID
		o, err = networkingsvc.CreateRouter(securityGroupRef.ResourceID)
		if err != nil {
			clusterScope.Error(err, "create router failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if o != nil {
			clusterScope.Info("router created", "id", qcs.StringValue(o))
			var g *qcs.DescribeRoutersOutput
			g, err = networkingsvc.GetRouter(o)
			if err != nil {
				return reconcile.Result{}, err
			}

			vpc.ResourceID = qcs.StringValue(o)
			vpcRef.ResourceID = qcs.StringValue(o)
			vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(g.RouterSet[0].Status))
		}
	} else if vpcRef.ResourceID != "" && vpc.ResourceID == "" {
		vpc.ResourceID = vpcRef.ResourceID
	} else {
		describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpc.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
		}
		vpcRef.ResourceID = qcs.StringValue(describeVPCOutput.RouterSet[0].RouterID)
		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	}

	// wait for vpc
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("vpc not ready")
		return reconcile.Result{}, errors.Errorf("unexpected vpc status: %v", vpcRef.ResourceStatus)
	}

	describeVxnetOutput, err := networkingsvc.GetVxNet(qcs.String(vxnetID))
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(describeVxnetOutput.VxNetSet) != 0 && qcs.StringValue(describeVxnetOutput.VxNetSet[0].VpcRouterID) == "" {
		// join vxnet to vpc
		if err = networkingsvc.JoinRouter(qcs.String(vpcRef.ResourceID), qcs.String(vxnetID), ipnetwork); err != nil {
			clusterScope.Error(err, "join router failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		qccluster.Status.Network.VxNetsRef[0].IPNetwork = ipnetwork
	}

	// wait for vpc
	describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("vpc not ready")
		return reconcile.Result{}, errors.Errorf("unexpected vpc status: %v", vpcRef.ResourceStatus)
	}

	// join EIP to vpc
	if qcs.StringValue(describeVPCOutput.RouterSet[0].EIP.EIPID) == "" {
		err = networkingsvc.BindEIP(qcs.String(eip.ResourceID), qcs.String(vpc.ResourceID))
		if err != nil {
			clusterScope.Error(err, "bind eip failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
	}

	// wait for vpc
	describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("vpc not ready")
		return reconcile.Result{}, errors.Errorf("unexpected vpc status: %v", vpcRef.ResourceStatus)
	}

	// create loadbalancer
	apiServerLoadbalancer := clusterScope.APIServerLoadbalancerSpec()
	apiServerLoadbalancerRef := clusterScope.APIServerLoadbalancerRef()
	loadbalancerIP := ""
	if apiServerLoadbalancerRef.ResourceID == "" && apiServerLoadbalancer.ResourceID == "" {
		var lbID infrav1beta1.QCResourceID
		lbID, err = networkingsvc.CreateLoadBalancer(qcs.String(vxnetID))
		if err != nil {
			clusterScope.Error(err, "create loadbalancer failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if lbID != nil {
			clusterScope.Info("loadbalancer created", "id", qcs.StringValue(lbID))
			apiServerLoadbalancer.ResourceID = qcs.StringValue(lbID)
			apiServerLoadbalancerRef.ResourceID = qcs.StringValue(lbID)
		}
	} else if apiServerLoadbalancerRef.ResourceID != "" && apiServerLoadbalancer.ResourceID == "" {
		apiServerLoadbalancer.ResourceID = apiServerLoadbalancerRef.ResourceID
	} else {
		apiServerLoadbalancerRef.ResourceID = apiServerLoadbalancer.ResourceID
	}

	// wait for loadbalancer
	l, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
	if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("loadbalancer not ready")
		return reconcile.Result{}, errors.Errorf("unexpected api server load balancer status: %v", apiServerLoadbalancerRef.ResourceStatus)
	}

	loadbalancerIP = qcs.StringValueSlice(l.LoadBalancerSet[0].PrivateIPs)[0]

	// create loadbalancer listener
	apiServerLoadbalancerListenRef := clusterScope.APIServerLoadbalancerListenerRef()
	if apiServerLoadbalancerListenRef.ResourceID == "" {
		var lbl *qcs.AddLoadBalancerListenersOutput
		lbl, err = networkingsvc.AddLoadBalancerListener(qcs.String(apiServerLoadbalancerRef.ResourceID))
		if err != nil {
			clusterScope.Error(err, "add loadbalancer listener failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		if lbl != nil {
			clusterScope.Info("loadbalancer listener added", "name", lbl.LoadBalancerListeners[0])
			apiServerLoadbalancerListenRef.ResourceID = qcs.StringValue(lbl.LoadBalancerListeners[0])
			apiServerLoadbalancerListenRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
		}
	}

	// wait for loadbalancer
	l, err = networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
	if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("loadbalancer not ready")
		return reconcile.Result{}, errors.Errorf("unexpected api server load balancer status: %v", apiServerLoadbalancerRef.ResourceStatus)
	}

	// get EIP Address
	qcsEIP, err := networkingsvc.GetEIP(qcs.String(eip.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	// set Port Forwarding
	controlPlanePort := int32(6443)
	if qccluster.Spec.ControlPlaneEndpoint.Port != 0 {
		controlPlanePort = qccluster.Spec.ControlPlaneEndpoint.Port
	}
	if eipRef.ResourceID == "" {
		err = networkingsvc.PortForwardingForEIP(strconv.FormatInt(int64(controlPlanePort), 10), loadbalancerIP, qcs.String(vpcRef.ResourceID))
		if err != nil {
			clusterScope.Error(err, "port forwarding for eip failed")
			if !qcerrors.IsAlreadyExisted(err) {
				return reconcile.Result{}, err
			}
		}
		eipRef.ResourceID = eip.ResourceID
	}
	// wait for vpc
	describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("vpc not ready")
		return reconcile.Result{}, errors.Errorf("unexpected vpc status: %v", vpcRef.ResourceStatus)
	}

	// set cluster control plane endpoint
	clusterScope.SetControlPlaneEndpoint(clusterv1beta1.APIEndpoint{
		Host: qcs.StringValue(qcsEIP.EIPAddr),
		Port: controlPlanePort,
	})

	clusterScope.Info("Set QCCluster status to ready")
	clusterScope.SetReady()

	r.Recorder.Eventf(qccluster, corev1.EventTypeNormal, "QCClusterReady", "QCCluster %s - has ready status", clusterScope.Name())
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QCClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter:             defaultControllerRateLimiter,
		}).
		For(&infrav1beta1.QCCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(context.TODO()))). // don't queue reconcile if resource is paused
		Build(r)
	if err != nil {
		return errors.Wrap(err, "error creating controller")
	}
	// Add a watch on clusterv1.Cluster object for unpause notifications.
	if err = c.Watch(
		&source.Kind{Type: &clusterv1beta1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(infrav1beta1.GroupVersion.WithKind("QCCluster"))),
		predicates.ClusterUnpaused(ctrl.LoggerFrom(context.TODO())),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

func (r *QCClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	clusterScope.Info("Reconciling delete QCCluster")
	qccluster := clusterScope.QCCluster
	networkingsvc := networking.NewService(ctx, clusterScope)
	vpcRef := clusterScope.RouterRef()

	// delete loadbalancer
	apiServerLoadbalancerRef := clusterScope.APIServerLoadbalancerRef()
	if apiServerLoadbalancerRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
		if err := networkingsvc.DeleteLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID)); err != nil {
			clusterScope.Error(err, "delete load balancer failed")
			if !qcerrors.IsNotFound(err) {
				apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
				return reconcile.Result{}, err
			}
		}
		apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
	}

	// wait for loadbalancer
	l, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		clusterScope.Error(err, "get load balancer failed")
		if !qcerrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
	}
	if l != nil {
		apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
		if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusDeleted && apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusCeased {
			clusterScope.Info("loadbalancer is not being deleted")
			return reconcile.Result{}, errors.Errorf("unexpected api server load balancer status: %v", apiServerLoadbalancerRef.ResourceStatus)
		}
	}

	//delete VxNet
	vxnetsRef := clusterScope.VxNetRef()
	for index, vxnetRef := range vxnetsRef {
		if vxnetRef.ResourceRef.ResourceStatus != infrav1beta1.QCResourceStatusDeleted {
			var describeVxnetOutput *qcs.DescribeVxNetsOutput
			describeVxnetOutput, err = networkingsvc.GetVxNet(qcs.String(vxnetRef.ResourceRef.ResourceID))
			if err != nil {
				clusterScope.Error(err, "get vxnet failed")
				if qcerrors.IsNotFound(err) {
					continue
				}
				return reconcile.Result{}, err
			}

			if len(describeVxnetOutput.VxNetSet) != 0 && qcs.StringValue(describeVxnetOutput.VxNetSet[0].VpcRouterID) != "" {
				if err = networkingsvc.LeaveRouter(qcs.String(vpcRef.ResourceID), qcs.String(vxnetRef.ResourceRef.ResourceID)); err != nil {
					clusterScope.Error(err, "leave router failed")
					// do not return err intentionally, we want to continue anyway
				}
				if err = networkingsvc.DeleteVxNet(qcs.String(vxnetRef.ResourceRef.ResourceID)); err != nil {
					clusterScope.Error(err, "delete vxnet failed")
					if !qcerrors.IsNotFound(err) {
						return reconcile.Result{}, err
					}
				}

				qccluster.Status.Network.VxNetsRef[index].ResourceRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleted
			}
		}
	}
	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusClear

	if clusterScope.GetVPCReclaimPolicy() == infrav1beta1.ReclaimDelete {
		if err = networkingsvc.DissociateEIP(qcs.String(qccluster.Status.Network.EIPRef.ResourceID)); err != nil {
			clusterScope.Error(err, "dissociate eip failed")
			// do not return err intentionally, we want to continue anyway
		}
		if err = networkingsvc.DeleteRouter(qcs.String(vpcRef.ResourceID)); err != nil {
			clusterScope.Error(err, "delete router failed")
			if !qcerrors.IsNotFound(err) {
				vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
				return reconcile.Result{}, err
			}
		}
		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing

		// wait for vpc
		var describeRoutersOutput *qcs.DescribeRoutersOutput
		describeRoutersOutput, err = networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
		}

		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeRoutersOutput.RouterSet[0].Status))
		if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusDeleted && vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusCeased {
			clusterScope.Info("vpc is not being deleted")
			return reconcile.Result{}, errors.Errorf("unexpected vpc status: %v", vpcRef.ResourceStatus)
		}

		// delete EIP
		eipRef := clusterScope.EIPRef()
		if eipRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
			if err = networkingsvc.DeleteEIP(qcs.String(eipRef.ResourceID)); err != nil {
				clusterScope.Error(err, "delete eip failed")
				if !qcerrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}
			eipRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
		}

		// delete SecurityGroup
		securityGroupRef := clusterScope.SecurityGroupRef()
		if securityGroupRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
			if err = networkingsvc.DeleteSecurityGroup(qcs.String(securityGroupRef.ResourceID)); err != nil {
				clusterScope.Error(err, "delete security group failed")
				if !qcerrors.IsNotFound(err) {
					return reconcile.Result{}, err
				}
			}
			securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
		}
	}
	controllerutil.RemoveFinalizer(qccluster, infrav1beta1.ClusterFinalizer)
	return reconcile.Result{}, nil
}
