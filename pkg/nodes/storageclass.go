package nodes

import (
	"bytes"
	"context"
	"html/template"
	"strings"

	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/clusterclient"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/deployments"
	"github.com/kubesphere/cluster-api-provider-qingcloud/pkg/installer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type QingCloudCSIData struct {
	QCAccessKeyID string
	QCAccessKey   string
	Zone          string
}

func StorageStatus(ctx context.Context, clients *clusterclient.ClusterClients) bool {
	QCCSI, err := clients.ClientSet.StorageV1().StorageClasses().Get(ctx, "csi-qingcloud", metav1.GetOptions{})
	if err != nil {
		return false
	}
	if QCCSI.Name == "csi-qingcloud" {
		return true
	}
	return false
}

func InstallQCCSI(ctx context.Context, clients *clusterclient.ClusterClients, qCClients *scope.QCClients, zone string) error {
	QCAccessKeyID := qCClients.SecurityGroupService.Config.AccessKeyID
	QCAccessKey := qCClients.SecurityGroupService.Config.SecretAccessKey

	QCCSIData := QingCloudCSIData{
		QCAccessKeyID: QCAccessKeyID,
		QCAccessKey:   QCAccessKey,
		Zone:          zone,
	}

	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}

	// create QingCloudCSI configmap
	klog.Info("starting install QingCloud CSI")
	var buf bytes.Buffer
	csiTempl, err := template.New("qingcloudcsi").Parse(deployments.QingCloudCSITemp)
	if err != nil {
		return err
	}
	if err := csiTempl.Execute(&buf, QCCSIData); err != nil {
		return err
	}
	QingCloudCSIConfigMap := buf.Bytes()

	if err := installerClient.Install(string(QingCloudCSIConfigMap)); err != nil {
		return err
	}

	// create QingCloudCSI
	qingCloudCSIResources := deployments.QingCloudCSIReources
	qingCloudCSIResource := strings.Split(string(qingCloudCSIResources), "---")
	for _, resource := range qingCloudCSIResource {
		if err := installerClient.Install(resource); err != nil {
			return err
		}
	}
	return nil
}

func UninstallQCCSI(ctx context.Context, clients *clusterclient.ClusterClients, qCClients *scope.QCClients) error {
	QCAccessKeyID := qCClients.SecurityGroupService.Config.AccessKeyID
	QCAccessKey := qCClients.SecurityGroupService.Config.SecretAccessKey
	Zone := qCClients.SecurityGroupService.Config.Zone

	QCCSIData := QingCloudCSIData{
		QCAccessKeyID: QCAccessKeyID,
		QCAccessKey:   QCAccessKey,
		Zone:          Zone,
	}

	installerClient := &installer.InstallerConfig{
		CTX:           ctx,
		KubeConfig:    clients.KubeConfig,
		ClientSet:     clients.ClientSet,
		DynamicClient: clients.DynamicClient,
	}

	// create QingCloudCSI configmap
	klog.Info("starting uninstall QingCloud CSI")
	var buf bytes.Buffer
	csiTempl, err := template.New("qingcloudcsi").Parse(deployments.QingCloudCSITemp)
	if err != nil {
		return err
	}
	if err := csiTempl.Execute(&buf, QCCSIData); err != nil {
		return err
	}
	QingCloudCSIConfigMap := buf.Bytes()

	if err := installerClient.Uninstall(string(QingCloudCSIConfigMap)); err != nil {
		return err
	}

	// create QingCloudCSI
	qingCloudCSIResources := deployments.QingCloudCSIReources
	qingCloudCSIResource := strings.Split(string(qingCloudCSIResources), "---")
	for _, resource := range qingCloudCSIResource {
		if err := installerClient.Uninstall(resource); err != nil {
			return err
		}
	}
	return nil
}
