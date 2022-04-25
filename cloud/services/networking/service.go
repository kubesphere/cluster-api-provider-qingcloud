package networking

import (
	"context"
	"github.com/kubesphere/cluster-api-provider-qingcloud/cloud/scope"
)

// Service holds a collection of interfaces.
type Service struct {
	scope *scope.ClusterScope
	ctx   context.Context
}

// NewService returns a new service given the qingcloud api client.
func NewService(ctx context.Context, scope *scope.ClusterScope) *Service {
	return &Service{
		scope: scope,
		ctx:   ctx,
	}
}
