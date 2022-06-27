package clusterclient

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type ClusterClients struct {
	Config        clientcmd.ClientConfig
	KubeConfig    *rest.Config
	ClientSet     kubernetes.Interface
	DynamicClient dynamic.Interface
	MetricsClient metricsv.Interface
}

func GetClusterClients(secret *corev1.Secret) (*ClusterClients, error) {
	clients := new(ClusterClients)
	clusterKubeconfigValue := secret.Data["value"]
	config, err := GetConfigFromBytyes(clusterKubeconfigValue)
	if err != nil {
		return nil, err
	}
	clients.Config = config
	restConfig, err := GetKubeConfig(config)
	if err != nil {
		return nil, err
	}
	clients.KubeConfig = restConfig
	clientSet, err := GetClientset(restConfig)
	if err != nil {
		return nil, err
	}
	clients.ClientSet = clientSet
	dynamicClient, err := GetDynamicClient(restConfig)
	if err != nil {
		return nil, err
	}
	clients.DynamicClient = dynamicClient
	metricsClient, err := GetMetricsClient(restConfig)
	if err != nil {
		return nil, err
	}
	clients.MetricsClient = metricsClient
	return clients, nil
}

func GetKubeConfig(config clientcmd.ClientConfig) (*rest.Config, error) {
	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}
	return restConfig, nil
}

func GetClientset(restConfig *rest.Config) (*kubernetes.Clientset, error) {
	clientsetForCluster, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return clientsetForCluster, nil
}

func GetDynamicClient(restConfig *rest.Config) (DynamicClient dynamic.Interface, err error) {
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

func GetConfigFromBytyes(in []byte) (clientcmd.ClientConfig, error) {
	config, err := clientcmd.NewClientConfigFromBytes(in)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func GetMetricsClient(restConfig *rest.Config) (metricsv.Interface, error) {
	metricsClient, err := metricsv.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return metricsClient, nil
}
