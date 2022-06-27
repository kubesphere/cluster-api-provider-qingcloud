package clusterautoscale

import (
	"context"
	"fmt"
	"math"

	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/clusterclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NodeMetricsInfo map[string]metricsv1beta1.NodeMetrics
type NodeCPUUtilization struct {
	name                  string
	currentCPUUtilization int64
	targetCPUUtilization  int32
}
type CurrentNodeUtilization map[string]int

type ReplicaCalculator struct {
	client            client.Client
	machineScope      *scope.MachineScope
	machineDeployment *clusterv1beta1.MachineDeployment
	clusterclients    *clusterclient.ClusterClients
}

func NewReplicaCalculator(client client.Client, machineScope *scope.MachineScope, machineDeployment *clusterv1beta1.MachineDeployment, clusterclients *clusterclient.ClusterClients) *ReplicaCalculator {
	return &ReplicaCalculator{
		client:            client,
		machineScope:      machineScope,
		machineDeployment: machineDeployment,
		clusterclients:    clusterclients,
	}
}

func (c *ReplicaCalculator) AutoScaleClusterReplicase(ctx context.Context) error {
	machineDeployment := c.machineDeployment.DeepCopy()

	desiredReplicas, rescale, rescaleReason, err := c.CalcPlainQCMachineReplicas(ctx, machineDeployment)
	if err != nil {
		return err
	}
	if rescale {
		status := "Successful"
		message := "AutoScaleMachineDeployment"
		severity := clusterv1beta1.ConditionSeverityInfo
		machineDeployment.Spec.Replicas = &desiredReplicas
		machineDeployment.Status.Conditions = setconditions(machineDeployment.Status.Conditions, severity, rescaleReason, message, status)
		klog.Infof("start scale machineDeployment replicas to %v", &desiredReplicas)
		if err := c.client.Update(ctx, machineDeployment); err != nil {
			return err
		}
	}
	return nil
}

func (c *ReplicaCalculator) CalcPlainQCMachineReplicas(ctx context.Context, machineDeployment *clusterv1beta1.MachineDeployment) (desiredReplicas int32, rescale bool, rescaleReason string, err error) {
	qcCluster := c.machineScope.QCCluster.DeepCopy()
	reference := fmt.Sprintf("%s/%s/%s", machineDeployment.GetObjectKind(), machineDeployment.GetNamespace(), machineDeployment.GetName())
	currentReplicas := machineDeployment.Spec.Replicas
	minReplicas := qcCluster.Spec.ClusterAutoScale.MinReplicas
	maxReplicas := qcCluster.Spec.ClusterAutoScale.MaxReplicas
	targetCPUUtilizationPercentage := qcCluster.Spec.ClusterAutoScale.TargetCPUUtilizationPercentage
	if *currentReplicas == 0 && minReplicas == 0 {
		desiredReplicas = 0
		rescale = false
	} else if minReplicas > maxReplicas {
		return *currentReplicas, false, "", err
	} else if *currentReplicas > maxReplicas {
		desiredReplicas = maxReplicas
		rescaleReason = "Current number of replicas above Spec.ClusterAutoScale.MaxReplicas"
	} else if *currentReplicas < minReplicas {
		desiredReplicas = minReplicas
		rescaleReason = "Current number of replicas below Spec.ClusterAutoScale.MinReplicas"
	} else {
		newReplicas, err := c.calcPlainMetricReplicas(ctx, *currentReplicas, targetCPUUtilizationPercentage)
		if err != nil {
			return 0, false, "", err
		}
		klog.V(4).Infof("proposing %v desired replicas (based on metrics) for %s", newReplicas, reference)
		if newReplicas > *currentReplicas {
			desiredReplicas = newReplicas
			rescaleReason = "cluster CPU Utilization Percentage above Spec.ClusterAutoScale.TargetCPUUtilizationPercentage"
		} else if newReplicas > maxReplicas {
			desiredReplicas = maxReplicas
		} else {
			desiredReplicas = *currentReplicas
		}
		rescale = desiredReplicas != *currentReplicas
	}
	return
}

func (c *ReplicaCalculator) calcPlainMetricReplicas(ctx context.Context, currentReplicas int32, targetCPUUtilizationPercentage int32) (replicaCount int32, err error) {
	// machineList, err := c.ListMachineBylabel(ctx)
	nodeList, err := c.getNodeList(ctx)
	if err != nil {
		return currentReplicas, nil
	}

	nodeMetrics, err := c.getNodeMetrics(ctx)
	if err != nil {
		return currentReplicas, err
	}
	nodeMetricsInfo := c.getNodeMetricsInfo(nodeMetrics)

	readyMachineCount, unreadyMachines, ignoredMachines, controlPlaneMachines := groupNodes(nodeList)
	nodeMetricsInfo = removeMetricsForNodes(nodeMetricsInfo, unreadyMachines)
	nodeMetricsInfo = removeMetricsForNodes(nodeMetricsInfo, ignoredMachines)
	nodeMetricsInfo = removeMetricsForNodes(nodeMetricsInfo, controlPlaneMachines)
	if len(nodeMetricsInfo) == 0 {
		return currentReplicas, fmt.Errorf("did not receive metrics for any ready nodes")
	}

	nodeCPUUtilizationList, err := c.getMetricUtilizationRatio(ctx, nodeMetricsInfo, targetCPUUtilizationPercentage)
	if err != nil {
		return currentReplicas, err
	}

	rebalanceIgnored := len(unreadyMachines) > 0

	var nodeCPUUtilization NodeCPUUtilization
	if rebalanceIgnored {
		for unreadyMachine := range unreadyMachines {
			nodeCPUUtilization.name = unreadyMachine
			nodeCPUUtilization.currentCPUUtilization = 0
			nodeCPUUtilizationList = append(nodeCPUUtilizationList, nodeCPUUtilization)
		}
	} else {
		clusterCUPUtilization := c.getCPUUtilizationTotal(nodeCPUUtilizationList)
		newReplicas := int32(math.Ceil(float64(clusterCUPUtilization) / float64(currentReplicas) / float64(targetCPUUtilizationPercentage) * float64(readyMachineCount)))

		if newReplicas < currentReplicas {
			return currentReplicas, nil
		}
		return newReplicas, nil
	}

	clusterCUPUtilization := c.getCPUUtilizationTotal(nodeCPUUtilizationList)
	newReplicas := int32(math.Ceil(float64(int(clusterCUPUtilization/int64(currentReplicas)) * len(nodeCPUUtilizationList))))
	if newReplicas < currentReplicas {
		return currentReplicas, nil
	}
	return newReplicas, nil
}

func (c *ReplicaCalculator) ListMachineBylabel(ctx context.Context) (*clusterv1beta1.MachineList, error) {
	cluster := c.machineScope.Cluster
	labels := map[string]string{clusterv1beta1.ClusterLabelName: cluster.Name}
	machineList := &clusterv1beta1.MachineList{}
	if err := c.client.List(ctx, machineList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, nil
	}
	return machineList, nil
}

func (c *ReplicaCalculator) getNodeList(ctx context.Context) (*corev1.NodeList, error) {
	clientset := c.clusterclients.ClientSet
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nodeList, nil
}

func (c *ReplicaCalculator) getNodeMetrics(ctx context.Context) (nodeMetrics *metricsv1beta1.NodeMetricsList, err error) {
	metricsClient := c.clusterclients.MetricsClient
	if err != nil {
		return nil, err
	}
	nodeMetrics, err = metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return
}

func (c *ReplicaCalculator) getNodeMetricsInfo(nodeMetrics *metricsv1beta1.NodeMetricsList) NodeMetricsInfo {
	nodeMetricsInfo := make(NodeMetricsInfo)
	for _, metricsItem := range nodeMetrics.Items {
		nodeMetricsInfo[metricsItem.Name] = metricsItem
	}
	return nodeMetricsInfo
}

func (c *ReplicaCalculator) getMetricUtilizationRatio(ctx context.Context, nodeMetricsInfo NodeMetricsInfo, targetUtilization int32) ([]NodeCPUUtilization, error) {
	nodeCPUUtilization := new(NodeCPUUtilization)
	nodeCPUUtilizationList := new([]NodeCPUUtilization)
	for nodeName, metrics := range nodeMetricsInfo {
		nodeDetail, err := c.clusterclients.ClientSet.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		nodeCPUCapacity := nodeDetail.Status.Capacity.Cpu().MilliValue()
		nodeCPUUsage := metrics.Usage.Cpu().MilliValue()
		CurrentCPUUtilization := (nodeCPUUsage * 100) / nodeCPUCapacity

		nodeCPUUtilization.name = nodeName
		nodeCPUUtilization.currentCPUUtilization = CurrentCPUUtilization
		nodeCPUUtilization.targetCPUUtilization = targetUtilization
		*nodeCPUUtilizationList = append(*nodeCPUUtilizationList, *nodeCPUUtilization)
	}
	return *nodeCPUUtilizationList, nil
}

func (c *ReplicaCalculator) getCPUUtilizationTotal(nodeCPUUtilizationList []NodeCPUUtilization) (CPUUtilizationTotal int64) {
	total := int64(0)
	for _, CPUUtilization := range nodeCPUUtilizationList {
		total += CPUUtilization.currentCPUUtilization
	}
	CPUUtilizationTotal = total / int64(len(nodeCPUUtilizationList))
	return
}

func groupNodes(nodeList *corev1.NodeList) (readyNodeCount int, unreadyNodes, ignoredNodes, controlPlaneNodes sets.String) {
	unreadyNodes = sets.NewString()
	ignoredNodes = sets.NewString()
	controlPlaneNodes = sets.NewString()
	for _, node := range nodeList.Items {
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			controlPlaneNodes.Insert(node.Name)
		} else {
			if node.DeletionTimestamp != nil || node.Status.Phase == corev1.NodeTerminated {
				ignoredNodes.Insert(node.Name)
			}

			if node.Status.Phase == corev1.NodePending {
				unreadyNodes.Insert(node.Name)
			}

			if node.Status.Phase == corev1.NodeRunning || node.Status.Phase == "" {
				readyNodeCount++
			}
		}
	}
	return
}

func removeMetricsForNodes(nodeMetricsInfo NodeMetricsInfo, nodes sets.String) NodeMetricsInfo {
	for _, node := range nodes.UnsortedList() {
		delete(nodeMetricsInfo, node)
	}
	return nodeMetricsInfo
}

func setconditions(inputConditions clusterv1beta1.Conditions, severity clusterv1beta1.ConditionSeverity, reason string, message string, status string) clusterv1beta1.Conditions {
	var existCond bool
	conditionType := clusterv1beta1.ConditionType("autoScale")
	newCondition := clusterv1beta1.Condition{
		Type:               conditionType,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Severity:           severity,
		Status:             corev1.ConditionStatus(status),
		Message:            message,
	}
	for _, condition := range inputConditions {
		if condition.Type == conditionType {
			condition = newCondition
			existCond = true
		}
	}
	if !existCond {
		inputConditions = append(inputConditions, newCondition)
	}
	return inputConditions
}
