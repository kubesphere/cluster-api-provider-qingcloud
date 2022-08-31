package scope

import (
	"os"

	"github.com/yunify/qingcloud-sdk-go/config"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

// QCClients hold all necessary clients to work with the QingCloud API.
type QCClients struct {
	Instance             *qcs.InstanceService
	Router               *qcs.RouterService
	VxNet                *qcs.VxNetService
	SecurityGroupService *qcs.SecurityGroupService
	LoadBalancer         *qcs.LoadBalancerService
	EIP                  *qcs.EIPService
	UserData             *qcs.UserDataService
}

// NewQCClients create QingCloud Client, set the zone of the resources
func NewQCClients(zone string) (*QCClients, error) {
	// By deployment env, set ACCESS_KEY_ID and SECRET_ACCESS_KEY in secret as environments.
	configuration, err := config.New(os.Getenv("ACCESS_KEY_ID"), os.Getenv("SECRET_ACCESS_KEY"))
	if err != nil {
		return nil, err
	}
	qcClient, err := qcs.Init(configuration)
	if err != nil {
		return nil, err
	}

	// set the zone of instances
	instance, err := qcClient.Instance(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of the router
	router, err := qcClient.Router(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of the vxNet
	vxnet, err := qcClient.VxNet(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of the security group
	securityGroup, err := qcClient.SecurityGroup(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of the loadBalance
	loadbalancer, err := qcClient.LoadBalancer(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of EIP
	eip, err := qcClient.EIP(zone)
	if err != nil {
		return nil, err
	}

	// set the zone of user
	userData, err := qcClient.UserData(zone)
	if err != nil {
		return nil, err
	}
	return &QCClients{
		Instance:             instance,
		Router:               router,
		VxNet:                vxnet,
		SecurityGroupService: securityGroup,
		LoadBalancer:         loadbalancer,
		EIP:                  eip,
		UserData:             userData,
	}, nil
}
