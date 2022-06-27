package nodes

import (
	"context"
	"strings"

	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/clusterclient"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/deployments"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/installer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func IngressStatus(ctx context.Context, clients *clusterclient.ClusterClients) bool {
	rs, err := clients.ClientSet.CoreV1().Namespaces().Get(ctx, "nginx-ingress", metav1.GetOptions{})
	if err != nil {
		return false
	}
	if rs.Name == "nginx-ingress" {
		return true
	}
	return false
}

func InstallIngress(ctx context.Context, clients *clusterclient.ClusterClients) error {
	ingressResources := deployments.IngressResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting install nginx-ingress")
	ingressResource := strings.Split(string(ingressResources), "---")
	for _, resource := range ingressResource {
		if err := installerClient.Install(resource); err != nil {
			return err
		}
	}
	return nil
}

func UninstallIngress(ctx context.Context, clients *clusterclient.ClusterClients) error {
	ingressResources := deployments.IngressResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting uninstall ingress")
	ingressResource := strings.Split(string(ingressResources), "---")
	for _, resource := range ingressResource {
		if err := installerClient.Uninstall(resource); err != nil {
			return err
		}
	}
	return nil
}
