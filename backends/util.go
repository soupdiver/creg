package backends

import (
	"fmt"
	"log"
	"strings"
)

func ExtractPorts(labels map[string]string, prefix string) map[string]string {
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

func MapServices(ports map[string]string, containerLabels map[string]string, staticLabelsToAdd []string, filters []FilterFunc) map[string]ServiceWithLabels {
	servicesByPort := map[string]ServiceWithLabels{}

	// map ports to services and labels
	for port, service := range ports {
		// clone the static labels to avoid overwriting them
		ls := append([]string(nil), staticLabelsToAdd...)

		for _, filter := range filters {
			filter(&ls, containerLabels, service)
		}

		servicesByPort[port] = ServiceWithLabels{
			Name:   service,
			Labels: ls,
		}
	}

	return servicesByPort
}

type FilterFunc func(serviceLabels *[]string, containerLabels map[string]string, service string)

func TraefikLabelFilter(serviceLabels *[]string, containerLabels map[string]string, service string) {
	for k, v := range containerLabels {
		if strings.Contains(k, "."+service) || strings.Contains(k, "traefik.") {
			*serviceLabels = append(*serviceLabels, fmt.Sprintf("%s=%s", k, v))
		}
	}
}
