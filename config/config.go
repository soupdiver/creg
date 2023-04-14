package config

import (
	consulapi "github.com/hashicorp/consul/api"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Config struct {
	ForwardAddress string
	StaticLabels   []string

	ConsulConfig *consulapi.Config
	EtcdConfig   *clientv3.Config
}

var DefaultConfig = Config{
	ConsulConfig: consulapi.DefaultConfig(),
	EtcdConfig:   &clientv3.Config{},
}
