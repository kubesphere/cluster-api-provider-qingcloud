package networking

import (
	"fmt"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	utilerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

// CreateRouter creates a Router for cluster.
func (s *Service) CreateRouter(securityGroupID string) (infrav1beta1.QCResourceID, error) {
	c := s.scope.Router
	clusterName := infrav1beta1.QCSafeName(s.scope.Name())
	name := fmt.Sprintf("%s-%s", clusterName, s.scope.UID())

	o, err := c.CreateRouters(
		&qcs.CreateRoutersInput{
			RouterName:    qcs.String(name),
			VpcNetwork:    qcs.String("192.168.0.0/16"),
			SecurityGroup: qcs.String(securityGroupID),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o.Routers[0], nil
}

// GetRouter get a Router by Router ID.
func (s *Service) GetRouter(routerID infrav1beta1.QCResourceID) (*qcs.DescribeRoutersOutput, error) {
	if qcs.StringValue(routerID) == "" {
		return nil, nil
	}

	o, err := s.scope.Router.DescribeRouters(
		&qcs.DescribeRoutersInput{
			Routers: []*string{routerID},
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return nil, utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o, nil
}

func (s *Service) JoinRouter(routerID, vxnetID infrav1beta1.QCResourceID, ipNetwork string) error {
	o, err := s.scope.Router.JoinRouter(
		&qcs.JoinRouterInput{
			IPNetwork: qcs.String(ipNetwork),
			Router:    routerID,
			VxNet:     vxnetID,
		},
	)

	if err != nil {
		return err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return nil
}

func (s *Service) UnbindingRouterVxnets(routerID infrav1beta1.QCResourceID, vxnetsRef []infrav1beta1.VxNetRef) error {
	c := s.scope.Router

	vxnets := []*string{}
	for _, vxnet := range vxnetsRef {
		vxnets = append(vxnets, &vxnet.ResourceRef.ResourceID)
	}

	l, err := c.LeaveRouter(
		&qcs.LeaveRouterInput{
			Router: routerID,
			VxNets: vxnets,
		},
	)

	if err != nil {
		return err
	}
	if qcs.IntValue(l.RetCode) != 0 {
		return utilerrors.NewQingCloudError(l.RetCode, l.Message)
	}

	//a, err := c.UpdateRouters(
	//	&qcs.UpdateRoutersInput{Routers: []*string{routerID}},
	//)
	//if err != nil {
	//	return err
	//}
	//if qcs.IntValue(a.RetCode) != 0 {
	//	return errors.New(qcs.StringValue(a.Message))
	//}
	return nil

}

func (s *Service) DeleteRoute(routerID infrav1beta1.QCResourceID) error {
	c := s.scope.Router
	d, err := c.DeleteRouters(
		&qcs.DeleteRoutersInput{Routers: []*string{routerID}},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(d.RetCode) != 0 {
		return utilerrors.NewQingCloudError(d.RetCode, d.Message)
	}
	return nil

}
