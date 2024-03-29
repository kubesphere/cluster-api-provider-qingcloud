package networking

import (
	"fmt"

	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	qcerrors "github.com/kubesphere/cluster-api-provider-qingcloud/util/errors/qingcloud"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

// GetLoadBalancer get a LB by LB ID.
func (s *Service) GetLoadBalancer(lbID infrav1beta1.QCResourceID) (*qcs.DescribeLoadBalancersOutput, error) {
	if qcs.StringValue(lbID) == "" {
		return nil, nil
	}

	o, err := s.scope.LoadBalancer.DescribeLoadBalancers(
		&qcs.DescribeLoadBalancersInput{
			LoadBalancers: []*string{lbID},
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

// CreateLoadBalancer creates a LB for cluster.
func (s *Service) CreateLoadBalancer(vxnetID infrav1beta1.QCResourceID) (infrav1beta1.QCResourceID, error) {
	c := s.scope.LoadBalancer
	clusterName := infrav1beta1.QCSafeName(s.scope.Name())
	name := fmt.Sprintf("%s-%s-%s", clusterName, infrav1beta1.APIServerRoleTagValue, s.scope.UID())

	o, err := c.CreateLoadBalancer(
		&qcs.CreateLoadBalancerInput{
			LoadBalancerName: qcs.String(name),
			VxNet:            vxnetID,
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, qcerrors.NewQingCloudError(o.RetCode, o.Message)
	}

	return o.LoadBalancerID, nil
}

// AddLoadBalancerListener add a kube-apiserver listener for LB.
func (s *Service) AddLoadBalancerListener(lbID infrav1beta1.QCResourceID) (*qcs.AddLoadBalancerListenersOutput, error) {
	c := s.scope.LoadBalancer

	l, err := c.AddLoadBalancerListeners(
		&qcs.AddLoadBalancerListenersInput{
			Listeners: []*qcs.LoadBalancerListener{
				&qcs.LoadBalancerListener{
					BackendProtocol:          qcs.String("tcp"),
					ListenerPort:             qcs.Int(6443),
					ListenerProtocol:         qcs.String("tcp"),
					LoadBalancerListenerName: qcs.String("kube-apiserver"),
				},
			},
			LoadBalancer: lbID,
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(l.RetCode) != 0 {
		return nil, qcerrors.NewQingCloudError(l.RetCode, l.Message)
	}

	u, err := c.UpdateLoadBalancers(
		&qcs.UpdateLoadBalancersInput{LoadBalancers: []*string{lbID}},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(u.RetCode) != 0 {
		return nil, qcerrors.NewQingCloudError(u.RetCode, u.Message)
	}

	return l, nil
}

func (s *Service) AddLoadBalancerBackend(lbID, lbListenerID, instanceID infrav1beta1.QCResourceID) error {
	c := s.scope.LoadBalancer

	b, err := c.AddLoadBalancerBackends(
		&qcs.AddLoadBalancerBackendsInput{
			Backends: []*qcs.LoadBalancerBackend{
				&qcs.LoadBalancerBackend{
					LoadBalancerBackendName: qcs.String("kube-apiserver"),
					Port:                    qcs.Int(6443),
					ResourceID:              instanceID,
				},
			},
			LoadBalancerListener: lbListenerID,
		},
	)

	if err != nil {
		return err
	}
	if qcs.IntValue(b.RetCode) != 0 {
		return qcerrors.NewQingCloudError(b.RetCode, b.Message)
	}

	u, err := c.UpdateLoadBalancers(
		&qcs.UpdateLoadBalancersInput{LoadBalancers: []*string{lbID}},
	)
	if err != nil {
		return err
	}
	if qcs.IntValue(u.RetCode) != 0 {
		return qcerrors.NewQingCloudError(u.RetCode, u.Message)
	}
	return nil
}

// DeleteLoadBalancer creates a LB for cluster.
func (s *Service) DeleteLoadBalancer(loadbalancerID infrav1beta1.QCResourceID) error {
	c := s.scope.LoadBalancer

	o, err := c.DeleteLoadBalancers(
		&qcs.DeleteLoadBalancersInput{
			LoadBalancers: []*string{loadbalancerID},
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
