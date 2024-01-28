package debug

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/soupdiver/creg/backends"
	"github.com/soupdiver/creg/types"
)

type Backend struct {
	log *logrus.Entry
}

func New(context.Context) backends.Backend {
	return &Backend{
		log: logrus.WithField("backend", "debug"),
	}
}

func (b *Backend) GetName() string {
	return "debug"
}

func (b *Backend) Run(ctx context.Context, events chan types.ContainerEventV2, purgeOnStart bool, containersToRefresh []types.ContainerInfo) error {
	for {
		select {
		case <-ctx.Done():
			b.log.Info("context cancelled")
			return nil
		case event := <-events:
			b.log.Infof("Event: %v", event.Action)
		}
	}
}

func (b *Backend) Purge() error {
	b.log.Info("purge")
	return nil
}

func (b *Backend) Refresh(containers []types.ContainerInfo) error {
	b.log.Info("refresh")
	return nil
}
