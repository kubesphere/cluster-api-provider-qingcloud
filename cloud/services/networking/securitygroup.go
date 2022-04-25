package networking

import (
	"fmt"
	infrav1beta1 "github.com/kubesphere/cluster-api-provider-qingcloud/api/v1beta1"
	"github.com/pkg/errors"
	qcs "github.com/yunify/qingcloud-sdk-go/service"
)

// CreateSecurityGroup creates a SecurityGroup for cluster.
func (s *Service) CreateSecurityGroup() (infrav1beta1.QCResourceID, error) {
	c := s.scope.SecurityGroup
	clusterName := infrav1beta1.QCSafeName(s.scope.Name())
	name := fmt.Sprintf("%s-%s", clusterName, s.scope.UID())

	o, err := c.CreateSecurityGroup(
		&qcs.CreateSecurityGroupInput{
			SecurityGroupName: qcs.String(name),
		},
	)
	if err != nil {
		return nil, err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(o.Message))
	}

	return o.SecurityGroupID, nil
}

// AddSecurityGroupRules adds SecurityGroupRules for cluster.
func (s *Service) AddSecurityGroupRules(securityGroupID infrav1beta1.QCResourceID, rules []*qcs.SecurityGroupRule) (infrav1beta1.QCResourceID, error) {
	c := s.scope.SecurityGroup
	o, err := c.AddSecurityGroupRules(
		&qcs.AddSecurityGroupRulesInput{
			Rules:         rules,
			SecurityGroup: securityGroupID,
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(o.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(o.Message))
	}

	a, err := c.ApplySecurityGroup(
		&qcs.ApplySecurityGroupInput{
			SecurityGroup: securityGroupID,
		},
	)
	if err != nil {
		return nil, err
	}
	if qcs.IntValue(a.RetCode) != 0 {
		return nil, errors.New(qcs.StringValue(a.Message))
	}

	return o.SecurityGroupRules[0], nil
}

// GetSecurityGroup get a SecurityGroup by SecurityGroup ID.
func (s *Service) GetSecurityGroup(securityGroupID infrav1beta1.QCResourceID) (*qcs.DescribeSecurityGroupsOutput, error) {
	if qcs.StringValue(securityGroupID) == "" {
		return nil, nil
	}

	o, err := s.scope.SecurityGroup.DescribeSecurityGroups(
		&qcs.DescribeSecurityGroupsInput{
			SecurityGroups: []*string{securityGroupID},
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

// DeleteSecurityGroup  creates a LB for cluster.
func (s *Service) DeleteSecurityGroup(securityGroupID infrav1beta1.QCResourceID) error {
	c := s.scope.SecurityGroup

	o, err := c.DeleteSecurityGroups(
		&qcs.DeleteSecurityGroupsInput{
			SecurityGroups: []*string{securityGroupID},
		},
	)
	if err != nil {
		return err
	}

	if qcs.IntValue(o.RetCode) != 0 {
		return errors.New(qcs.StringValue(o.Message))
	}

	return nil
}
