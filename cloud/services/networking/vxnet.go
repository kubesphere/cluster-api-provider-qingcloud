package networking

import (
	"fmt"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	qcerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors/qingcloud"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

// CreateVxNet creates a VxNet for cluster.
func (s *Service) CreateVxNet() (infrav1beta1.QCResourceID, error) {
	c := s.scope.VxNet
	clusterName := infrav1beta1.QCSafeName(s.scope.Name())
	name := fmt.Sprintf("%s-%s", clusterName, s.scope.UID())

	o, err := c.CreateVxNets(
		&qcs.CreateVxNetsInput{
			VxNetName: qcs.String(name),
			VxNetType: qcs.Int(1),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, qcerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o.VxNets[0], nil
}

// CreateVxNet gets a VxNet for cluster.
func (s *Service) GetVxNet(vxnetID infrav1beta1.QCResourceID) (*qcs.DescribeVxNetsOutput, error) {
	c := s.scope.VxNet

	o, err := c.DescribeVxNets(
		&qcs.DescribeVxNetsInput{
			VxNets:    []*string{vxnetID},
			VxNetType: qcs.Int(1),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, qcerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o, nil
}

// DeleteVxNet delete a vxnet for cluster.
func (s *Service) DeleteVxNet(vxnetID infrav1beta1.QCResourceID) error {
	c := s.scope.VxNet

	o, err := c.DeleteVxNets(
		&qcs.DeleteVxNetsInput{
			VxNets: []*string{vxnetID},
		},
	)
	if err != nil {
		return err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return qcerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return nil
}

// LeaveRouter delete a vxnet from vpc.
func (s *Service) LeaveRouter(routerID, vxnetID infrav1beta1.QCResourceID) error {
	r := s.scope.Router

	l, err := r.LeaveRouter(
		&qcs.LeaveRouterInput{
			Router: routerID,
			VxNets: []*string{vxnetID},
		},
	)
	if err != nil {
		return err
	}

	if qcs.IntValue(l.RetCode) != 0 {
		return qcerrors.NewQingCloudError(l.RetCode, l.Message)
	}

	return nil
}
