package backends

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/soupdiver/creg/docker"
)

type Backend interface {
	Run(ctx context.Context, events chan docker.ContainerEvent, purgeOnStart bool, containersToRefresh []types.ContainerJSON) error
	Purge() error
	Refresh(containers []types.ContainerJSON) error
}

type ServiceWithLabels struct {
	Name   string
	Labels []string
}

const ServicePrefix = "creg"
const ServiceLabelPort = "creg.port"

type BackendOption func(*Backend)
