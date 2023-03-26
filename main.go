package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/docker/docker/client"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/soupdiver/creg/backends/consul"
	"github.com/soupdiver/creg/docker"
	flag "github.com/spf13/pflag"
)

var (
	fAddress       = flag.String("address", "", "Address to use for consul services")
	fConsulAddress = flag.String("consul", "", "Address of consul agent")
	fHelp          = flag.BoolP("help", "h", false, "Print usage")
	fLabels        = flag.StringSliceP("labels", "l", []string{}, "Labels to append tp consul services")
	fCleanOnStart  = flag.Bool("clean", false, "Clean consul services on start")
)

var (
	DefaultLabels = []string{"dc=remote"}
	CREDLabel     = "consul-reg.port"
)

func main() {
	if err := Run(); err != nil {
		log.Fatalf("Fatal: %s", err)
	}
}

func Run() error {
	flag.Parse()

	if *fHelp {
		flag.PrintDefaults()
		return nil
	}

	if fAddress == nil || *fAddress == "" {
		return fmt.Errorf("address is required")
	}

	if fConsulAddress == nil || *fConsulAddress == "" {
		return fmt.Errorf("consul address is required")
	}

	if len(*fLabels) > 0 {
		for _, v := range *fLabels {
			split := strings.Split(v, "=")
			if len(split) != 2 {
				continue
			}
		}
	}

	// Prepare root context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Consul client and backend
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = *fConsulAddress
	consulClient, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return fmt.Errorf("could not create consul client: %w", err)
	}
	consulBackend := consul.New(consulClient)

	if *fCleanOnStart {
		err := consulBackend.Purge()
		if err != nil {
			return fmt.Errorf("could not clean consul services: %w", err)
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		consulBackend.Run(ctx, docker.GetEventsForCreg(ctx, dockerClient, "container-reg"), []string{"dc=remote"})
	}()

	c := make(chan os.Signal, 5)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		log.Printf("Received %s, cancelling context", s)
		cancel()
	}()

	wg.Wait()

	log.Printf("Done!")

	return nil
}
