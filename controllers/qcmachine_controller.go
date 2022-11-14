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
	"time"

	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/services/compute"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/services/networking"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/clusterclient"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/nodes"
	qcerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors/qingcloud"
	"github.com/pkg/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// QCMachineReconciler reconciles a QCMachine object
type QCMachineReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=qcmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the QCMachine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *QCMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	//ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	//defer cancel()

	log := ctrl.LoggerFrom(ctx).WithName(req.NamespacedName.String())

	qcMachine := &infrav1beta1.QCMachine{}

	if err := r.Get(ctx, req.NamespacedName, qcMachine); err != nil {
		log.Error(err, "get qc machine failed")
		if kubeerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, qcMachine.ObjectMeta)
	if err != nil {
		log.Error(err, "get owner machine failed")
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Error(err, "get cluster failed")
		return ctrl.Result{}, err
	}

	qcCluster := &infrav1beta1.QCCluster{}
	qcClusternamespacedName := client.ObjectKey{
		Namespace: qcMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}

	if err = r.Get(ctx, qcClusternamespacedName, qcCluster); err != nil {
		log.Error(err, "get qc cluster failed")
		return ctrl.Result{}, err
	}

	// Return early if the object or Cluster is paused.
	if annotations.IsPaused(cluster, qcCluster) {
		log.Info("QCMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	qcClients, err := scope.NewQCClients(qcCluster.Spec.Zone)
	if err != nil {
		log.Error(err, "get qc clients failed")
		return ctrl.Result{}, err
	}

	// create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		QCClients: *qcClients,
		Client:    r.Client,
		Logger:    log,
		Cluster:   cluster,
		QCCluster: qcCluster,
	})
	if err != nil {
		log.Error(err, "get cluster scope failed")
		return ctrl.Result{}, err
	}

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		QCClients: *qcClients,
		Client:    r.Client,
		Logger:    log,
		Cluster:   cluster,
		Machine:   machine,
		QCCluster: qcCluster,
		QCMachine: qcMachine,
	})
	if err != nil {
		log.Error(err, "get machine scope failed")
		return ctrl.Result{}, err
	}

	// Always close the scope when exiting this function so we can persist any QCMachine changes.
	defer func() {
		if err = machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// handle deleted machines
	if !qcMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope, clusterScope)
	}

	return r.reconcile(ctx, machineScope, clusterScope)
}

// SetupWithManager sets up the controller with the Manager.
func (r *QCMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
			RateLimiter:             defaultControllerRateLimiter,
		}).
		For(&infrav1beta1.QCMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(ctrl.LoggerFrom(context.TODO()))).
		Watches(
			&source.Kind{Type: &clusterv1beta1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1beta1.GroupVersion.WithKind("QCMachine"))),
		).
		Watches(
			&source.Kind{Type: &infrav1beta1.QCCluster{}},
			handler.EnqueueRequestsFromMapFunc(r.QCClusterToQCmachines(context.TODO())),
		).
		Build(r)
	if err != nil {
		return errors.Wrapf(err, "error creating controller")
	}

	clusterToObjectFunc, err := util.ClusterToObjectsMapper(r.Client, &infrav1beta1.QCClusterList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to QCMachines")
	}

	// Add a watch on clusterv1.Cluster object for unpause & ready notifications.
	if err := c.Watch(
		&source.Kind{Type: &clusterv1beta1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
		predicates.ClusterUnpausedAndInfrastructureReady(ctrl.LoggerFrom(context.TODO())),
	); err != nil {
		return errors.Wrapf(err, "failed adding a watch for ready clusters")
	}

	return nil
}

// QCClusterToQCmachines convert the cluster to machines spec.
func (r *QCMachineReconciler) QCClusterToQCmachines(ctx context.Context) handler.MapFunc {
	log := ctrl.LoggerFrom(ctx)
	return func(o client.Object) []ctrl.Request {
		result := []ctrl.Request{}

		c, ok := o.(*infrav1beta1.QCCluster)
		if !ok {
			log.Error(errors.Errorf("expected a QCCluster but got a %T", o), "failed to get QCMachine for QCCluster")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case kubeerrors.IsNotFound(err) || cluster == nil:
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster")
			return result
		}

		labels := map[string]string{clusterv1beta1.ClusterLabelName: cluster.Name}
		machineList := &clusterv1beta1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines")
			return nil
		}

		for _, m := range machineList.Items {
			if m.Spec.InfrastructureRef.Name == "" {
				continue
			}
			name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
			result = append(result, ctrl.Request{NamespacedName: name})
		}
		return result
	}
}

func (r *QCMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	machineScope.Info("Reconciling delete QCMachine")
	qcMachine := machineScope.QCMachine

	computesvc := compute.NewService(ctx, clusterScope)
	instance, err := computesvc.GetInstance(machineScope.GetInstanceID())
	if err != nil && !qcerrors.IsNotFoundOrDeleted(err) {
		machineScope.Error(err, "get instance failed")
		return ctrl.Result{}, err
	}

	if instance != nil {
		err = computesvc.DeleteInstance(machineScope.GetInstanceID())
		if err != nil && !qcerrors.IsNotFoundOrDeleted(err) {
			machineScope.Error(err, "delete instance failed")
			return ctrl.Result{}, err
		}
	} else {
		clusterScope.V(2).Info("Unable to locate instance")
		r.Recorder.Eventf(qcMachine, corev1.EventTypeWarning, "NoInstanceFound", "Skip deleting")
	}

	r.Recorder.Eventf(qcMachine, corev1.EventTypeNormal, "InstanceDeleted", "Deleted a instance - %s", machineScope.Name())
	controllerutil.RemoveFinalizer(qcMachine, infrav1beta1.MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *QCMachineReconciler) reconcile(ctx context.Context, machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	machineScope.Info("Reconciling QCMachine (Instance)")
	qcMachine := machineScope.QCMachine

	// If the QCMachine is in an error state, return early.
	if qcMachine.Status.FailureReason != nil || qcMachine.Status.FailureMessage != nil {
		machineScope.Info("Error state detected, skipping reconcile")
		return ctrl.Result{}, nil
	}

	// If the QCMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(qcMachine, infrav1beta1.MachineFinalizer)

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		return ctrl.Result{}, nil
	}

	computesvc := compute.NewService(ctx, clusterScope)
	instanceID := machineScope.GetInstanceID()
	instance, err := computesvc.GetInstance(instanceID)
	if err != nil {
		machineScope.Error(err, "get instance failed")
		return ctrl.Result{}, err
	}
	if instance == nil {
		if machineScope.QCMachine.Spec.SSHKeyID == nil {
			err = errors.Errorf("Not found SSHKeyID")
			machineScope.Error(err, "SSHKeyID not set in spec")
			r.Recorder.Event(qcMachine, corev1.EventTypeWarning, "NotFoundSSHKeyID", err.Error())
			machineScope.SetInstanceStatus(infrav1beta1.QCResourceStatusPending)
			return ctrl.Result{}, err
		}
		instanceID, err = computesvc.CreateInstance(machineScope)
		if err != nil {
			machineScope.Error(err, "create instance failed")
			err = errors.Errorf("Failed to create instance for QCMachine %s/%s: %v", qcMachine.Namespace, qcMachine.Name, err)
			r.Recorder.Event(qcMachine, corev1.EventTypeWarning, "InstanceCreatingError", err.Error())
			machineScope.SetInstanceStatus(infrav1beta1.QCResourceStatusCeased)
			return ctrl.Result{}, err
		}
		machineScope.SetProviderID(qcs.StringValue(instanceID))
		r.Recorder.Eventf(qcMachine, corev1.EventTypeNormal, "InstanceCreated", "Created new instance - %s", qcMachine.Name)
		instance, err = computesvc.GetInstance(instanceID)
		if err != nil {
			machineScope.Error(err, "get instance failed")
			return ctrl.Result{}, err
		}
	}
	machineScope.SetInstanceStatus(infrav1beta1.QCResourceStatus(qcs.StringValue(instance.InstanceSet[0].Status)))
	instanceState := *machineScope.GetInstanceStatus()
	switch instanceState {
	case infrav1beta1.QCResourceStatusPending:
		machineScope.Info("QCMachine instance is pending", "instance-id", *machineScope.GetInstanceID())
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	case infrav1beta1.QCResourceStatusRunning:
		machineScope.Info("QCMachine instance is running", "instance-id", *machineScope.GetInstanceID())
		r.Recorder.Eventf(qcMachine, corev1.EventTypeNormal, "QCMachine instance is running", "QCMachine instance is running - instance-id: %s", *machineScope.GetInstanceID())
		r.Recorder.Eventf(qcMachine, corev1.EventTypeNormal, "QCMachine instance is running", "QCMachine %s - has ready status", *machineScope.GetInstanceID())
		if !machineScope.QCMachine.Status.Ready && machineScope.Role() == infrav1beta1.APIServerRoleTagValue {
			networksvc := networking.NewService(ctx, clusterScope)
			if err = networksvc.AddLoadBalancerBackend(qcs.String(clusterScope.QCCluster.Status.Network.APIServerLoadbalancersRef.ResourceID), qcs.String(clusterScope.QCCluster.Status.Network.APIServerLoadbalancersListenerRef.ResourceID), instanceID); err != nil {
				machineScope.Error(err, "add instance to lb failed")
				if !qcerrors.IsAlreadyExisted(err) {
					return reconcile.Result{}, err
				}
			}
		}
		machineScope.SetReady()

		clusterKubeconfigSecret := &corev1.Secret{}
		clusterKubeconfigSecretKey := types.NamespacedName{Namespace: clusterScope.Cluster.Namespace, Name: clusterScope.Cluster.Name + "-kubeconfig"}
		if err = r.Client.Get(context.TODO(), clusterKubeconfigSecretKey, clusterKubeconfigSecret); err != nil {
			machineScope.Error(err, fmt.Sprintf("get secret %s failed", clusterScope.Cluster.Name+"-kubeconfig"))
			return ctrl.Result{}, err
		}
		var clusterClient *kubernetes.Clientset
		err, clusterClient = clusterclient.GetClusterClient(clusterKubeconfigSecret)
		if err != nil {
			machineScope.Error(err, "get cluster client failed")
			return ctrl.Result{}, err
		}

		_, err = clusterClient.CoreV1().Nodes().Get(context.TODO(), qcMachine.Name, metav1.GetOptions{})
		if err != nil {
			machineScope.Error(err, "get node failed")
			return ctrl.Result{}, err
		}

		if err = nodes.DeleteTaints(clusterClient, qcMachine); err != nil {
			machineScope.Error(err, "delete taints for node failed")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	default:
		machineScope.SetFailureReason(capierrors.UpdateMachineError)
		machineScope.SetFailureMessage(errors.Errorf("QCMachine instance state %s is unexpected", instanceState))
		return ctrl.Result{}, errors.Errorf("unexpected instance state: %v", instanceState)
	}
}
