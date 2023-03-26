package consul

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/soupdiver/creg/docker"
)

const ServicePrefix = "creg"
const ServiceLabelPort = "consul-reg.port"

type Backend struct {
	ConsulClient *consulapi.Client
}

func New(c *consulapi.Client) *Backend {
	return &Backend{
		ConsulClient: c,
	}
}

func (b *Backend) Run(context context.Context, events chan docker.ContainerEvent, staticLabelsToAdd []string) error {
	for {
		select {
		case <-context.Done():
			return nil
		case event := <-events:
			switch event.Event.Action {
			case "start":
				ports := extractPorts(event.Container.Config.Labels, ServiceLabelPort)
				servicesByPort := mapServices(ports, event.Container.Config.Labels, staticLabelsToAdd, []FilterFunc{TraefikLabelFilter})
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

type ServiceWithLabels struct {
	Name   string
	Labels []string
}

func mapServices(ports map[string]string, containerLabels map[string]string, staticLabelsToAdd []string, filters []FilterFunc) map[string]ServiceWithLabels {
	servicesByPort := map[string]ServiceWithLabels{}

	// map ports to services and labels
	for port, service := range ports {
		// clone the static labels to avoid overwriting them
		ls := append([]string(nil), staticLabelsToAdd...)

		for _, filter := range filters {
			filter(ls, containerLabels, service)
		}

		servicesByPort[port] = ServiceWithLabels{
			Name:   service,
			Labels: ls,
		}
	}

	return servicesByPort
}

type FilterFunc func(serviceLabels []string, containerLabels map[string]string, service string)

func TraefikLabelFilter(serviceLabels []string, containerLabels map[string]string, service string) {
	for k, v := range containerLabels {
		if strings.Contains(k, "."+service) || strings.Contains(k, "traefik.enable") {
			serviceLabels = append(serviceLabels, fmt.Sprintf("%s=%s", k, v))
		}
	}
}

// functen that registgers ports to consul
func (b *Backend) RegisterServices(ports map[string]ServiceWithLabels) error {
	registration := &consulapi.AgentServiceRegistration{
		Address: "",
	}

	for port, service := range ports {
		log.Printf("adding: %s", port)
		registration.Tags = service.Labels
		registration.Port, _ = strconv.Atoi(port)
		registration.ID = service.Name
		registration.Name = service.Name

		// migrate to new service name
		err := b.ConsulClient.Agent().ServiceDeregister(registration.Name)
		if err != nil {
			log.Printf("error reregister: %s", err)
		}

		registration.Name = ServicePrefix + registration.Name

		err = b.ConsulClient.Agent().ServiceRegister(registration)
		if err != nil {
			log.Printf("error register: %s", err)
		}
	}

	return nil
}

func extractPorts(labels map[string]string, prefix string) map[string]string {
	ports := map[string]string{}

	for k, v := range labels {
		v := strings.Replace(v, "'", "", -1)
		if strings.HasPrefix(k, prefix) {
			splitP := strings.Split(v, ",")
			for _, v := range splitP {
				split := strings.Split(v, ":")
				if len(split) != 2 {
					log.Printf("not len 2 after port split: %s", v)
					continue
				}

				ports[split[0]] = split[1]
			}
		}
	}

	return ports
}

func (b *Backend) Purge() error {
	// retrieve all consul containers and delete the ones where name has ServerPrefix
	services, err := b.ConsulClient.Agent().Services()
	if err != nil {
		return fmt.Errorf("could not agent.Services: %w", err)
	}

	for name, service := range services {
		if strings.HasPrefix(name, ServicePrefix) {
			err := b.ConsulClient.Agent().ServiceDeregister(service.ID)
			if err != nil {
				log.Printf("Could not agent.ServiceDeregister: %s", err)
			}
		}
	}

	return nil
}
