package scope

import (
	"github.com/yunify/qingcloud-sdk-go/config"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
	"os"
)

// QCClients hold all necessary clients to work with the QingCloud API.
type QCClients struct {
	Instance      *qcs.InstanceService
	Router        *qcs.RouterService
	VxNet         *qcs.VxNetService
	SecurityGroup *qcs.SecurityGroupService
	LoadBalancer  *qcs.LoadBalancerService
	EIP           *qcs.EIPService
	UserData      *qcs.UserDataService
}

func NewQCClients(zone string) (*QCClients, error) {
	configuration, err := config.New(os.Getenv("ACCESS_KEY_ID"), os.Getenv("SECRET_ACCESS_KEY"))
	if err != nil {
		return nil, err
	}
	qcClient, err := qcs.Init(configuration)
	if err != nil {
		return nil, err
	}

	instance, err := qcClient.Instance(zone)
	if err != nil {
		return nil, err
	}

	router, err := qcClient.Router(zone)
	if err != nil {
		return nil, err
	}

	vxnet, err := qcClient.VxNet(zone)
	if err != nil {
		return nil, err
	}

	securityGroup, err := qcClient.SecurityGroup(zone)
	if err != nil {
		return nil, err
	}

	loadbalancer, err := qcClient.LoadBalancer(zone)
	if err != nil {
		return nil, err
	}

	eip, err := qcClient.EIP(zone)
	if err != nil {
		return nil, err
	}
	userData, err := qcClient.UserData(zone)
	if err != nil {
		return nil, err
	}
	return &QCClients{
		Instance:      instance,
		Router:        router,
		VxNet:         vxnet,
		SecurityGroup: securityGroup,
		LoadBalancer:  loadbalancer,
		EIP:           eip,
		UserData:      userData,
	}, nil
}
