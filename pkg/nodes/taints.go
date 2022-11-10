package nodes

import (
	"context"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/pkg/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

func DeleteTaints(client *kubernetes.Clientset, qcmachine *infrav1beta1.QCMachine) error {
	taints := []corev1.Taint{}
	nodeclient := client.CoreV1().Nodes()
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, getRrr := nodeclient.Get(context.TODO(), qcmachine.Name, metav1.GetOptions{})
		if getRrr != nil {
			return getRrr
		}
		if _, ok := node.Labels["topology.cluster-api-provider-qingcloud/instance-type"]; !ok {
			node.Labels["topology.cluster-api-provider-qingcloud/instance-type"] = qcs.StringValue(qcmachine.Spec.InstanceType)
			for _, taint := range node.Spec.Taints {
				if taint.Key == "node.cloudprovider.kubernetes.io/uninitialized" {
					break
				} else {
					taints = append(taints, taint)
				}
			}
			node.Spec.Taints = taints
		}
		_, updateErr := nodeclient.Update(context.TODO(), node, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		return errors.Wrap(retryErr, "Update node failed")
	}
	return nil
}
