package clusterclient

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetClusterClient(secret *corev1.Secret) (error, *kubernetes.Clientset) {
	clusterKubeconfigValue := secret.Data["value"]
	// kubeConfigStr, err := base64.StdEncoding.DecodeString(string(clusterKubeconfigValue))
	// if err != nil {
	//     return err, nil
	// }
	config, err := clientcmd.NewClientConfigFromBytes(clusterKubeconfigValue)
	if err != nil {
		return err, nil
	}
	restConfig, err := config.ClientConfig()
	if err != nil {
		return err, nil
	}
	clientsetForCluster, err := kubernetes.NewForConfig(restConfig)
	return nil, clientsetForCluster
}
