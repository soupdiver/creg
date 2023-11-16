package backends

import (
	"context"

	ctypes "github.com/soupdiver/creg/types"
)

type Backend interface {
	Run(ctx context.Context, events chan ctypes.ContainerEventV2, purgeOnStart bool, containersToRefresh []ctypes.ContainerInfo) error
	Purge() error
	Refresh(containers []ctypes.ContainerInfo) error
	GetName() string
}

type ServiceWithLabels struct {
	Name   string
	Labels []string
}

const ServicePrefix = "creg"
const ServiceLabelPort = "creg.port"

type BackendOption func(*Backend)
