package networking

import (
	"fmt"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	utilerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

const (
	BillModeBandwidth = "bandwidth"
	BillModeTraffic   = "traffic"
)

// CreateEIP creates a EIP for cluster.
func (s *Service) CreateEIP(bandWidth int, billMode string) (infrav1beta1.QCResourceID, error) {
	c := s.scope.EIP
	clusterName := infrav1beta1.QCSafeName(s.scope.Name())
	name := fmt.Sprintf("%s-%s", clusterName, s.scope.UID())

	o, err := c.AllocateEIPs(
		&qcs.AllocateEIPsInput{
			Bandwidth:   qcs.Int(bandWidth),
			BillingMode: qcs.String(billMode),
			EIPName:     qcs.String(name),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o.EIPs[0], nil
}

func (s *Service) BindEIP(eipID, routerID infrav1beta1.QCResourceID) error {
	c := s.scope.Router
	o, err := c.ModifyRouterAttributes(
		&qcs.ModifyRouterAttributesInput{
			EIP:    eipID,
			Router: routerID,
		},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	a, err := c.UpdateRouters(
		&qcs.UpdateRoutersInput{Routers: []*string{routerID}},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(a.RetCode) != 0 {
		return utilerrors.NewQingCloudError(a.RetCode, a.Message)
	}
	return nil
}

// GetEIP creates a EIP for cluster.
func (s *Service) GetEIP(eipID infrav1beta1.QCResourceID) (*qcs.EIP, error) {
	c := s.scope.EIP

	o, err := c.DescribeEIPs(
		&qcs.DescribeEIPsInput{
			EIPs: []*string{eipID},
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o.EIPSet[0], nil
}

func (s *Service) PortForwardingForEIP(contronPlanePort, intelIP string, routerID infrav1beta1.QCResourceID) error {
	c := s.scope.Router
	o, err := c.AddRouterStatics(
		&qcs.AddRouterStaticsInput{
			Router: routerID,
			Statics: []*qcs.RouterStatic{
				&qcs.RouterStatic{
					StaticType: qcs.Int(1),
					Val1:       qcs.String(contronPlanePort),
					Val2:       qcs.String(intelIP),
					Val3:       qcs.String("6443"),
				},
			},
		},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	a, err := c.UpdateRouters(
		&qcs.UpdateRoutersInput{Routers: []*string{routerID}},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(a.RetCode) != 0 {
		return utilerrors.NewQingCloudError(a.RetCode, a.Message)
	}
	return nil
}

// DissociateEIP dissociate a EIP for cluster.
func (s *Service) DissociateEIP(eipID infrav1beta1.QCResourceID) error {
	c := s.scope.EIP

	o, err := c.DissociateEIPs(
		&qcs.DissociateEIPsInput{EIPs: []*string{eipID}},
	)
	if err != nil {
		return err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return nil
}

// DeleteEIP delete a EIP for cluster.
func (s *Service) DeleteEIP(eipID infrav1beta1.QCResourceID) error {
	c := s.scope.EIP

	o, err := c.ReleaseEIPs(
		&qcs.ReleaseEIPsInput{EIPs: []*string{eipID}},
	)
	if err != nil {
		return err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return utilerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return nil
}
