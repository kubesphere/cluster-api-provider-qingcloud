package compute

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
	"github.com/pkg/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

// CreateInstance creates a Instance for cluster.
func (s *Service) CreateInstance(scope *scope.MachineScope) (infrav1beta1.QCResourceID, error) {
	s.scope.V(2).Info("Creating an instance for a machine")

	instanceName := infrav1beta1.QCSafeName(scope.Name())

	vxnets := []string{}
	for _, vxnet := range scope.QCCluster.Spec.Network.VxNets {
		vxnets = append(vxnets, vxnet.ResourceID)
	}

	userdata, err := s.UploadUserData(instanceName, scope)
	if err != nil {
		return nil, err
	}

	c := s.scope.Instance
	o, err := c.RunInstances(
		&qcs.RunInstancesInput{
			NeedUserdata:  qcs.Int(1),
			UserdataType:  qcs.String("tar"),
			UserdataValue: userdata.AttachmentID,
			ImageID:       scope.QCMachine.Spec.ImageID,
			InstanceName:  qcs.String(instanceName),
			InstanceType:  scope.QCMachine.Spec.InstanceType,
			LoginMode:     qcs.String("keypair"),
			LoginKeyPair:  scope.QCMachine.Spec.SSHKeyID,
			OSDiskSize:    scope.QCMachine.Spec.OSDiskSize,
			VxNets:        qcs.StringSlice(vxnets),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(o.Message))
	}

	return o.Instances[0], nil
}

// GetInstance get a Instance by Instance ID.
func (s *Service) GetInstance(instanceID infrav1beta1.QCResourceID) (*qcs.DescribeInstancesOutput, error) {
	if qcs.StringValue(instanceID) == "" {
		return nil, nil
	}

	o, err := s.scope.Instance.DescribeInstances(
		&qcs.DescribeInstancesInput{
			Instances: []*string{instanceID},
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(o.Message))
	}

	return o, nil
}

// UploadUserData upload userdata for Instance.
func (s *Service) UploadUserData(instanceName string, scope *scope.MachineScope) (*qcs.UploadUserDataAttachmentOutput, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	workDir := pwd + instanceName
	userDataName := fmt.Sprintf("%s.tar", instanceName)
	bootstrapData, err := scope.GetBootstrapData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode bootstrap data")
	}
	err = ConvertToUserData(bootstrapData, workDir, userDataName)
	if err != nil {
		return nil, err
	}

	userDdataPath := filepath.Join(pwd, userDataName)
	//fmt.Println(userDdataPath)
	//f, err := ioutil.ReadFile(userDdataPath)
	//if err != nil {
	//	panic(err)
	//}
	//attachmentContent := base64.RawStdEncoding.EncodeToString(f)
	//fmt.Println(attachmentContent)

	output, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("cat %s | base64 -w 0", userDdataPath)).Output()
	if err != nil {
		return nil, err
	}

	userDataClient := s.scope.UserData
	var userDataOutput *qcs.UploadUserDataAttachmentOutput
	userDataOutput, err = userDataClient.UploadUserDataAttachment(
		&qcs.UploadUserDataAttachmentInput{
			AttachmentContent: qcs.String(string(output)),
			AttachmentName:    qcs.String(userDataName),
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(userDataOutput.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(userDataOutput.Message))
	}

	err = os.RemoveAll(workDir)
	if err != nil {
		return nil, err
	}
	err = os.RemoveAll(filepath.Join(pwd, userDataName))
	if err != nil {
		return nil, err
	}

	return userDataOutput, nil
}

type CloudConfig struct {
	WriteFiles []WriteFile `json:"write_files" yaml:"write_files"`
	RunCMD     []string    `json:"runcmd" yaml:"runcmd"`
}
type WriteFile struct {
	Path        string `json:"path" yaml:"path"`
	Owner       string `json:"owner" yaml:"owner"`
	Permissions string `json:"permissions" yaml:"permissions"`
	Content     string `json:"content" yaml:"content"`
}

func ConvertToUserData(data []byte, workdir, tarName string) error {
	cloudConfig := &CloudConfig{}

	err := yaml.Unmarshal(data, cloudConfig)
	if err != nil {
		return err
	}
	configContent := ""
	for _, file := range cloudConfig.WriteFiles {
		configContent = configContent + WriteFileContent(file.Path, file.Content, file.Permissions, file.Owner)
	}

	err = os.MkdirAll(filepath.Join(workdir, "/etc"), os.ModePerm)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(workdir, "/etc/rc.local"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	bootstrapScript := "#!/bin/bash\n\n" +
		"sleep 20\n" + "swapoff -a\n" +
		"source /etc/qingcloud/userdata/metadata.env\n" +
		configContent

	for _, c := range cloudConfig.RunCMD {
		bootstrapScript = bootstrapScript + c
	}

	f.WriteString(bootstrapScript)

	err = Tar(workdir, filepath.Join(filepath.Dir(workdir), tarName), workdir)
	if err != nil {
		return err
	}
	return nil
}

func getPerm(s string) os.FileMode {
	switch s {
	case "0640":
		return 0640
	case "0600":
		return 0600
	default:
		return 0777
	}
}

func Tar(src, dst, trimPrefix string) error {
	fw, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fw.Close()

	gw := gzip.NewWriter(fw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		fr, err := os.Open(path)
		defer fr.Close()
		if err != nil {
			return err
		}

		path = strings.TrimPrefix(path, trimPrefix)
		fmt.Println(strings.TrimPrefix(path, string(filepath.Separator)))

		hdr.Name = strings.TrimPrefix(path, string(filepath.Separator))
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if _, err := io.Copy(tw, fr); err != nil {
			return err
		}

		return nil
	})
}

func WriteFileContent(file, context, perm, owner string) string {
	path, _ := filepath.Split(file)
	return fmt.Sprintf("mkdir -p %s\n", path) +
		fmt.Sprintf("cat > %s << EOF\n", file) +
		context + "\n" +
		"EOF\n" +
		fmt.Sprintf("chown %s %s\n", owner, file) +
		fmt.Sprintf("chmod %s %s\n", perm, file)
}

// DeleteInstance get a Instance by Instance ID.
func (s *Service) DeleteInstance(instanceID infrav1beta1.QCResourceID) error {
	s.scope.V(2).Info("Attempting to delete instance", "instance-id", qcs.StringValue(instanceID))
	if qcs.StringValue(instanceID) == "" {
		s.scope.Info("Instance does not have an instance id")
		return errors.New("cannot delete instance. instance does not have an instance id")
	}

	o, err := s.scope.Instance.TerminateInstances(
		&qcs.TerminateInstancesInput{
			Instances: []*string{instanceID},
		},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return errors.New(qcs.StringValue(o.Message))
	}
	s.scope.V(2).Info("Deleted instance", "instance-id", qcs.StringValue(instanceID))
	return nil
}
