package types

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type ContainerEvent struct {
	Message   events.Message
	Container types.ContainerJSON
}

type ContainerEventV2 struct {
	Action    string
	Container ContainerInfo
}

type ContainerInfo struct {
	ID              string
	Labels          map[string]string
	NetworkSettings NetworkSettings
}

type NetworkSettings struct {
	Ports map[Port][]PortBinding
}

// type PortBinding struct {
// 	HostIP   string
// 	HostPort string
// }

// PortBinding represents a binding between a Host IP address and a Host Port
type PortBinding struct {
	// HostIP is the host IP Address
	HostIP string `json:"HostIp"`
	// HostPort is the host port number
	HostPort string
}

// PortMap is a collection of PortBinding indexed by Port
type PortMap map[Port][]PortBinding

// PortSet is a collection of structs indexed by Port
type PortSet map[Port]struct{}

// Port is a string containing port number and protocol in the format "80/tcp"
type Port string

func (p Port) Proto() string {
	proto, _ := SplitProtoPort(string(p))
	return proto
}

// Port returns the port number of a Port
func (p Port) Port() string {
	_, port := SplitProtoPort(string(p))
	return port
}

type CregEventSource interface {
	GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ContainerEventV2
	GetEventsForCregV2(ctx context.Context, label string) chan ContainerEventV2
}

// SplitProtoPort splits a port in the format of proto/port
func SplitProtoPort(rawPort string) (string, string) {
	parts := strings.Split(rawPort, "/")
	l := len(parts)
	if len(rawPort) == 0 || l == 0 || len(parts[0]) == 0 {
		return "", ""
	}
	if l == 1 {
		return "tcp", rawPort
	}
	if len(parts[1]) == 0 {
		return "tcp", parts[0]
	}
	return parts[1], parts[0]
}
