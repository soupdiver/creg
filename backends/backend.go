package backends

import "github.com/docker/docker/api/types/events"

type Backend interface {
	Run(chan events.Message) error
	Purge() error
}
