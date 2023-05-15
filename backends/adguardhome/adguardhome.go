package adguardhome

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"

	"github.com/soupdiver/creg/adguardhome"
	"github.com/soupdiver/creg/adguardhome/client"
	"github.com/soupdiver/creg/docker"
)

type Backend struct {
	Client *client.Client
	Log    *logrus.Entry
}

func New(address, auth string) (*Backend, error) {
	b := &Backend{
		Client: client.New(address, auth),
	}

	return b, nil
}

func (b *Backend) Run(ctx context.Context, events chan docker.ContainerEvent, purgeOnStart bool, containersToRefresh []types.ContainerJSON) error {
	for {
		select {
		case <-ctx.Done():
			b.Log.Infof("AdguardHome exting: %s", "context cancelled")
			return nil
		case event := <-events:
			log.Printf("handle event adguardhome: %s", event.Event.Action)
			switch event.Event.Action {
			case "start":
				if v, ok := event.Container.Config.Labels["creg.dns"]; ok {
					sp := strings.Split(v, ",")
					b.Client.Add(adguardhome.RewriteListResponseItem{Domain: sp[0], Answer: sp[1]})
				}
			case "stop":
				if v, ok := event.Container.Config.Labels["creg.dns"]; ok {
					sp := strings.Split(v, ",")
					b.Client.Delete(adguardhome.RewriteListResponseItem{Domain: sp[0], Answer: sp[1]})
				}
			}
		}
	}
	return nil
}

func (b *Backend) GetName() string {
	return "adguardhome"
}

func (b *Backend) Purge() error {
	return errors.New("not implemented")
}

func (b *Backend) Refresh(in []types.ContainerJSON) error {
	return errors.New("not implemented")
}
