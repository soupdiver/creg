package types

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type ContainerEvent struct {
	Event     events.Message
	Container types.ContainerJSON
}

type ContainerEventV2 struct {
	Container ContainerInfo
}

type ContainerInfo struct {
	ID     string
	Labels map[string]string
}

type CregEventSource interface {
	GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ContainerEventV2
}
