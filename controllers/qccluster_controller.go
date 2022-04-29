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
	"time"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/services/networking"
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
	log := log.FromContext(ctx).WithValues("controlleer", "QCCluster")
	//ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	//defer cancel()

	qcCluster := &infrav1beta1.QCCluster{}
	if err := r.Get(ctx, req.NamespacedName, qcCluster); err != nil {
		if kubeerrors.IsNotFound(err) {
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
		log.Info("Cluster Controller has not yet set OwnerRef")
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
		Logger:    log,
		Cluster:   cluster,
		QCCluster: qcCluster,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
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

	// create SecurityGroup
	securityGroupRef := clusterScope.SecurityGroupRef()
	if securityGroupRef.ResourceID == "" {
		o, err := networkingsvc.CreateSecurityGroup()
		if err != nil {
			clusterScope.Info("create security group failed")
			return reconcile.Result{}, err
		}
		rules := []*qcs.SecurityGroupRule{
			&qcs.SecurityGroupRule{
				Action:                qcs.String("accept"),
				Direction:             qcs.Int(0),
				Priority:              qcs.Int(1),
				Protocol:              qcs.String("tcp"),
				SecurityGroupID:       o,
				SecurityGroupRuleName: qcs.String("k8s apiserver"),
				Val1:                  qcs.String("6443"),
			},
		}

		_, err = networkingsvc.AddSecurityGroupRules(o, rules)
		if err != nil {
			return reconcile.Result{}, err
		}

		s, err := networkingsvc.GetSecurityGroup(o)
		if err != nil {
			return reconcile.Result{}, err
		}

		securityGroupRef.ResourceID = qcs.StringValue(o)
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
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// create eip
	eipRef := clusterScope.EIPRef()
	eip := clusterScope.EIPSpec()
	if eip.ResourceID == "" && eipRef.ResourceID == "" {
		if eip.BillingMode == "" {
			eip.BillingMode = networking.BillModeTraffic
		}
		if eip.Bandwidth == 0 {
			eip.Bandwidth = 10
		}
		eipID, err := networkingsvc.CreateEIP(eip.Bandwidth, eip.BillingMode)
		if err != nil {
			clusterScope.Info("create eip failed")
			return reconcile.Result{}, err
		}
		eip.ResourceID = qcs.StringValue(eipID)
		eipRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
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
		vID, err := networkingsvc.CreateVxNet()
		if err != nil {
			clusterScope.Info("create vxnet failed")
			return reconcile.Result{}, err
		}
		qccluster.Spec.Network.VxNets = []infrav1beta1.QCVxNet{{qcs.StringValue(vID), ""}}
		qccluster.Status.Network.VxNetsRef = []infrav1beta1.VxNetRef{{"", infrav1beta1.QCResourceReference{qcs.StringValue(vID), infrav1beta1.QCResourceStatusActive}}}
		vxnetID = qcs.StringValue(vID)
	} else if vxnetRef.ResourceRef.ResourceID != "" || vxnetID == "" {
		qccluster.Spec.Network.VxNets = []infrav1beta1.QCVxNet{{vxnetRef.ResourceRef.ResourceID, vxnetRef.IPNetwork}}
		vxnetID = vxnetRef.ResourceRef.ResourceID
	} else {
		qccluster.Status.Network.VxNetsRef = []infrav1beta1.VxNetRef{{"", infrav1beta1.QCResourceReference{vxnetID, infrav1beta1.QCResourceStatusActive}}}
	}

	// create VPC
	vpc := clusterScope.RouterSpec()
	vpcRef := clusterScope.RouterRef()
	describeVPCOutput := &qcs.DescribeRoutersOutput{}
	if vpc.ResourceID == "" && vpcRef.ResourceID == "" {
		o, err := networkingsvc.CreateRouter(securityGroupRef.ResourceID)
		if err != nil {
			clusterScope.Info("create router failed")
			return reconcile.Result{}, err
		}

		g, err := networkingsvc.GetRouter(o)
		if err != nil {
			return reconcile.Result{}, err
		}

		vpc.ResourceID = qcs.StringValue(o)
		vpcRef.ResourceID = qcs.StringValue(o)
		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(g.RouterSet[0].Status))
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
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// join vxnet to vpc
	if vxnetRef.IPNetwork == "" {
		if err = networkingsvc.JoinRouter(qcs.String(vpcRef.ResourceID), qcs.String(vxnetID), ipnetwork); err != nil {
			clusterScope.Info("join router failed")
			return reconcile.Result{}, err
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
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// join EIP to vpc
	if qccluster.Status.Network.EIPRef.ResourceID == "" {
		time.Sleep(8 * time.Second)
		err := networkingsvc.BindEIP(qcs.String(eip.ResourceID), qcs.String(vpc.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
		}
		qccluster.Status.Network.EIPRef.ResourceID = eip.ResourceID
	}

	// wait for vpc
	describeVPCOutput, err = networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("vpc not ready")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// create loadbalancer
	apiServerLoadbalancer := clusterScope.APIServerLoadbalancerSpec()
	apiServerLoadbalancerRef := clusterScope.APIServerLoadbalancerRef()
	loadbalancerIP := ""
	if apiServerLoadbalancerRef.ResourceID == "" && apiServerLoadbalancer.ResourceID == "" {
		lbID, err := networkingsvc.CreateLoadBalancer(qcs.String(vxnetID))
		if err != nil {
			clusterScope.Info("create loadbalancer failed")
			return reconcile.Result{}, err
		}
		apiServerLoadbalancer.ResourceID = qcs.StringValue(lbID)
		apiServerLoadbalancerRef.ResourceID = qcs.StringValue(lbID)
		_, err = networkingsvc.GetLoadBalancer(lbID)
		if err != nil {
			return reconcile.Result{}, err
		}

		// wait for loadbalancer
		for i := 0; i < 5; i++ {
			l, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
			if err != nil {
				return reconcile.Result{}, err
			}

			if len(qcs.StringValueSlice(l.LoadBalancerSet[0].PrivateIPs)) != 0 {
				apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
				loadbalancerIP = qcs.StringValueSlice(l.LoadBalancerSet[0].PrivateIPs)[0]

				break
			}
		}

	} else if apiServerLoadbalancerRef.ResourceID != "" && apiServerLoadbalancer.ResourceID == "" {
		apiServerLoadbalancer.ResourceID = apiServerLoadbalancerRef.ResourceID
	} else {
		o, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancer.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
		}
		apiServerLoadbalancerRef.ResourceID = apiServerLoadbalancer.ResourceID
		apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(o.LoadBalancerSet[0].Status))
		loadbalancerIP = qcs.StringValueSlice(o.LoadBalancerSet[0].PrivateIPs)[0]
	}

	// wait for loadbalancer
	l, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
	if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("loadbalancer not ready")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// create loadbalancer listener
	apiServerLoadbalancerListenRef := clusterScope.APIServerLoadbalancerListenerRef()
	if apiServerLoadbalancerListenRef.ResourceID == "" {
		l, err := networkingsvc.AddLoadBalancerListener(qcs.String(apiServerLoadbalancerRef.ResourceID))
		if err != nil {
			clusterScope.Info("add loadbalancer listener failed")
			return reconcile.Result{}, err
		}
		apiServerLoadbalancerListenRef.ResourceID = qcs.StringValue(l.LoadBalancerListeners[0])
		apiServerLoadbalancerListenRef.ResourceStatus = infrav1beta1.QCResourceStatusActive
	}

	// wait for loadbalancer
	l, err = networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
	if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusActive {
		clusterScope.Info("loadbalancer not ready")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// get EIP Address
	qcsEIP, err := networkingsvc.GetEIP(qcs.String(eipRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	// set Port Forwarding
	if qccluster.Spec.ControlPlaneEndpoint.Host == "" {
		err = networkingsvc.PortForwardingForEIP(loadbalancerIP, qcs.String(vpcRef.ResourceID))
		if err != nil {
			return reconcile.Result{}, err
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
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// set cluster control plane endpoint
	clusterScope.SetControlPlaneEndpoint(clusterv1beta1.APIEndpoint{
		Host: qcs.StringValue(qcsEIP.EIPAddr),
		Port: 6443,
	})

	clusterScope.Info("Set QCCluster status to ready")
	clusterScope.SetReady()

	r.Recorder.Eventf(qccluster, corev1.EventTypeNormal, "QCClusterReady", "QCCluster %s - has ready status", clusterScope.Name())
	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QCClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := ctrl.NewControllerManagedBy(mgr).
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

	// delete loadbalancer
	apiServerLoadbalancerRef := clusterScope.APIServerLoadbalancerRef()
	if apiServerLoadbalancerRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
		if err := networkingsvc.DeleteLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID)); err != nil {
			apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
			return reconcile.Result{}, err
		}
		apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
	}

	// wait for loadbalancer
	l, err := networkingsvc.GetLoadBalancer(qcs.String(apiServerLoadbalancerRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}
	apiServerLoadbalancerRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(l.LoadBalancerSet[0].Status))
	if apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusDeleted && apiServerLoadbalancerRef.ResourceStatus != infrav1beta1.QCResourceStatusCeased {
		clusterScope.Info("loadbalancer being deleted")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// delete VPC
	vxnetsRef := clusterScope.VxNetRef()
	vpcRef := clusterScope.RouterRef()
	if vpcRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
		if err := networkingsvc.UnbindingRouterVxnets(qcs.String(qccluster.Status.Network.RouterRef.ResourceID), vxnetsRef); err != nil {
			vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusClear
			return reconcile.Result{}, err
		}
		time.Sleep(30 * time.Second)
		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusClear
	}

	if vpcRef.ResourceStatus == infrav1beta1.QCResourceStatusClear {
		_ = networkingsvc.DissociateEIP(qcs.String(qccluster.Status.Network.EIPRef.ResourceID))

		if err := networkingsvc.DeleteRoute(qcs.String(vpcRef.ResourceID)); err != nil {
			vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
			return reconcile.Result{}, err
		}
		vpcRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
	}

	// wait for vpc
	describeVPCOutput, err := networkingsvc.GetRouter(qcs.String(vpcRef.ResourceID))
	if err != nil {
		return reconcile.Result{}, err
	}

	vpcRef.ResourceStatus = infrav1beta1.QCResourceStatus(qcs.StringValue(describeVPCOutput.RouterSet[0].Status))
	if vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusDeleted && vpcRef.ResourceStatus != infrav1beta1.QCResourceStatusCeased {
		clusterScope.Info("vpc being deleted")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// delete EIP
	eipRef := clusterScope.EIPRef()
	if eipRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
		if err := networkingsvc.DeleteEIP(qcs.String(eipRef.ResourceID)); err != nil {
			return reconcile.Result{}, err
		}
		eipRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
	}

	// delete SecurityGroup
	securityGroupRef := clusterScope.SecurityGroupRef()
	if securityGroupRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
		if err := networkingsvc.DeleteSecurityGroup(qcs.String(securityGroupRef.ResourceID)); err != nil {
			return reconcile.Result{}, err
		}
		securityGroupRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
	}

	//delete VxNet
	for index, vxnetRef := range vxnetsRef {
		if vxnetRef.ResourceRef.ResourceStatus == infrav1beta1.QCResourceStatusActive {
			if err := networkingsvc.DeleteVxNet(qcs.String(vxnetRef.ResourceRef.ResourceID)); err != nil {
				//return reconcile.Result{}, err
			}
			qccluster.Status.Network.VxNetsRef[index].ResourceRef.ResourceStatus = infrav1beta1.QCResourceStatusDeleteing
		}
	}

	controllerutil.RemoveFinalizer(qccluster, infrav1beta1.ClusterFinalizer)
	return reconcile.Result{}, nil
}
