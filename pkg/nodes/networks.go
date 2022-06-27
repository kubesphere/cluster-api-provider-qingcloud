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

func CalicoStatus(ctx context.Context, clients *clusterclient.ClusterClients) bool {
	deploy, err := clients.ClientSet.AppsV1().Deployments("kube-system").Get(ctx, "calico-kube-controllers", metav1.GetOptions{})
	if err != nil {
		return false
	}
	if deploy.Name == "calico-kube-controllers" {
		return true
	}
	return false
}

func InstallCalico(ctx context.Context, clients *clusterclient.ClusterClients) error {
	calicoResources := deployments.CalicoResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}

	klog.Info("starting install calico")
	calicoResource := strings.Split(string(calicoResources), "---")
	for _, resource := range calicoResource {
		if err := installerClient.Install(resource); err != nil {
			return err
		}
	}

	return nil
}

func UninstallCalico(ctx context.Context, clients *clusterclient.ClusterClients) error {
	calicoResources := deployments.CalicoResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}

	klog.Info("starting uninstall calico")
	calicoResource := strings.Split(string(calicoResources), "---")
	for _, resource := range calicoResource {
		if err := installerClient.Uninstall(resource); err != nil {
			return err
		}
	}

	return nil
}
