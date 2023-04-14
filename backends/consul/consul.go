package consul

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	consulapi "github.com/hashicorp/consul/api"

	"github.com/soupdiver/creg/backends"
	"github.com/soupdiver/creg/docker"
)

type Backend struct {
	ConsulClient   *consulapi.Client
	ForwardAddress string
	StaticLabels   []string
}

func New(cfg *consulapi.Config, options ...ConsulOption) (*Backend, error) {
	b := &Backend{}

	for _, option := range options {
		option(b)
	}

	consulClient, err := consulapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not create consul client: %w", err)
	}
	b.ConsulClient = consulClient

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
				err := b.ConsulClient.Agent().ServiceDeregister(event.Container.ID)
				if err != nil {
					log.Printf("Could not DeregisterServices: %s", err)
					continue
				}
			}
		}
	}
}

// functen that registgers ports to consul
func (b *Backend) RegisterServices(ports map[string]backends.ServiceWithLabels) error {
	for port, service := range ports {
		registration := &consulapi.AgentServiceRegistration{
			ID:      backends.ServicePrefix + "-" + service.Name,
			Name:    backends.ServicePrefix + "-" + service.Name,
			Address: b.ForwardAddress,
			Tags:    service.Labels,
		}

		log.Printf("adding: %s", port)
		var err error
		registration.Port, err = strconv.Atoi(strings.Split(port, "/")[0])
		if err != nil {
			log.Printf("error parsing port: %s", err)
			continue
		}

		// err = b.ConsulClient.Agent().ServiceDeregister(service.Name)
		// if err != nil {
		// 	log.Printf("error deregister: %s", err)
		// }

		err = b.ConsulClient.Agent().ServiceRegister(registration)
		if err != nil {
			log.Printf("error register: %s", err)
		}
	}

	return nil
}

func (b *Backend) Purge() error {
	log.Printf("Purging consul services")
	// retrieve all consul containers and delete the ones where name has ServerPrefix
	services, err := b.ConsulClient.Agent().Services()
	if err != nil {
		return fmt.Errorf("could not agent.Services: %w", err)
	}

	for name, service := range services {
		log.Printf("purge service: %s - %s", name, service.ID)
		if strings.HasPrefix(name, backends.ServicePrefix) {
			err := b.ConsulClient.Agent().ServiceDeregister(service.ID)
			if err != nil {
				log.Printf("Could not agent.ServiceDeregister: %s", err)
			} else {
				log.Printf("Deregistered service: %s", service.ID)
			}
		}
	}

	return nil
}

func (b *Backend) Refresh(containers []types.ContainerJSON) error {
	log.Printf("Refreshing %d consul services", len(containers))
	for _, container := range containers {
		// log.Printf("container labels: %+v", container.Config.Labels)
		ports := backends.ExtractPorts(container.Config.Labels, backends.ServiceLabelPort)

		for port, info := range container.NetworkSettings.Ports {
			if v, ok := ports[port.Port()+"/"+port.Proto()]; ok {
				ports[info[0].HostPort] = v
				delete(ports, port.Port()+"/"+port.Proto())
			}
		}

		servicesByPort := backends.MapServices(ports, container.Config.Labels, []string{"dc=remote"}, []backends.FilterFunc{backends.TraefikLabelFilter})
		// log.Printf("Registering services: %+v", servicesByPort)
		err := b.RegisterServices(servicesByPort)
		if err != nil {
			log.Printf("Could not RegisterServices: %s", err)
			continue
		}
	}

	return nil
}

type ConsulOption func(*Backend)

func WithStaticLabels(labels []string) func(b *Backend) {
	return func(b *Backend) {
		b.StaticLabels = labels
	}
}
