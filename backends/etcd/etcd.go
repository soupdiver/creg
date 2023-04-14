package etcd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/soupdiver/creg/backends"
	"github.com/soupdiver/creg/docker"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Backend struct {
	EtcdClient     *clientv3.Client
	ForwardAddress string
	StaticLabels   []string
}

type EtcdOption func(*Backend)

func New(cfg clientv3.Config, options ...EtcdOption) (*Backend, error) {
	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create etcd client: %w", err)
	}

	b := &Backend{
		EtcdClient: c,
	}

	for _, option := range options {
		option(b)
	}

	return b, nil
}

func (b *Backend) Run(ctx context.Context, events chan docker.ContainerEvent, purgeOnStart bool, containersToRefresh []types.ContainerJSON) error {
	var err error
	if purgeOnStart {
		err = b.Purge()
		if err != nil {
			return fmt.Errorf("could not purge: %w", err)
		}

	}

	if len(containersToRefresh) > 0 {
		err = b.Refresh(containersToRefresh)
		if err != nil {
			return fmt.Errorf("could not refresh: %w", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-events:
			switch event.Event.Action {
			case "start":
				ports := backends.ExtractPorts(event.Container.Config.Labels, backends.ServiceLabelPort)
				servicesByPort := backends.MapServices(ports, event.Container.Config.Labels, b.StaticLabels, []backends.FilterFunc{backends.TraefikLabelFilter})
				err := b.RegisterServices(servicesByPort)
				if err != nil {
					log.Printf("Could not RegisterServices: %s", err)
					continue
				}
			case "stop":
				// err = b.Purge()
				// if err != nil {
				// 	log.Printf("could not purge: %w", err)
				// }
			}
		}
	}
}

func (b *Backend) Purge() error {
	// Use the etcd client to delete the key-value pair
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.EtcdClient.Delete(ctx, backends.ServicePrefix+"/", clientv3.WithPrefix())
	if err != nil {
		return err
	}

	return nil
}

func (b *Backend) Refresh(containers []types.ContainerJSON) error {
	return nil
}

func (b *Backend) RegisterServices(ports map[string]backends.ServiceWithLabels) error {
	var err error
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	for port, service := range ports {
		// Use the etcd client to put the key-value pair
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = b.EtcdClient.Put(ctx, backends.ServicePrefix+"/"+service.Name+"/"+hostname, fmt.Sprintf("%s:%s", b.ForwardAddress, port))
		if err != nil {
			return err
		}
	}

	return nil
}

func WithStaticLabels(labels []string) func(b *Backend) {
	return func(b *Backend) {
		b.StaticLabels = labels
	}
}
