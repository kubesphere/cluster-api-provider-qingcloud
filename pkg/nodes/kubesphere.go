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

func KubeSphereStatus(ctx context.Context, clients *clusterclient.ClusterClients) bool {
	rs, err := clients.ClientSet.CoreV1().Namespaces().Get(ctx, "kubesphere-system", metav1.GetOptions{})
	if err != nil {
		return false
	}
	if rs.Name == "kubesphere-system" {
		return true
	}
	return false
}

func InstallKubesphere(ctx context.Context, clients *clusterclient.ClusterClients) error {
	KubeSphereResources := deployments.KubeSphereResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting install KubeSphere")
	KubeSphereResource := strings.Split(string(KubeSphereResources), "---")
	for _, resource := range KubeSphereResource {
		if err := installerClient.Install(resource); err != nil {
			return err
		}
	}
	return nil
}

func UninstallKubeSphere(ctx context.Context, clients *clusterclient.ClusterClients) error {
	KubeSphereResources := deployments.KubeSphereResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting uninstall KubeSphere")
	KubeSphereResource := strings.Split(string(KubeSphereResources), "---")
	for _, resource := range KubeSphereResource {
		if err := installerClient.Uninstall(resource); err != nil {
			return err
		}
	}
	return nil
}
