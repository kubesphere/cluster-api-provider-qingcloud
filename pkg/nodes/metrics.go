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

func MetricsStatus(ctx context.Context, clients *clusterclient.ClusterClients) bool {
	metricsClient := clients.MetricsClient

	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return false
	}
	if len(nodeMetrics.Items) > 0 {
		return true
	}
	return false
}

func InstallMetrics(ctx context.Context, clients *clusterclient.ClusterClients) error {
	metricsResources := deployments.MetricsResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting install calico")
	metricsResource := strings.Split(string(metricsResources), "---")
	for _, resource := range metricsResource {
		if err := installerClient.Install(resource); err != nil {
			return err
		}
	}
	return nil
}

func UnInstallMetrics(ctx context.Context, clients *clusterclient.ClusterClients) error {
	metricsResource := deployments.MetricsResources
	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}
	klog.Info("starting uninstall calico")
	if err := installerClient.Uninstall(string(metricsResource)); err != nil {
		return err
	}
	return nil
}
