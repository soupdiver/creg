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
	flag "github.com/spf13/pflag"

	"github.com/soupdiver/creg/backends"
	"github.com/soupdiver/creg/backends/consul"
	"github.com/soupdiver/creg/backends/etcd"
	"github.com/soupdiver/creg/config"
	"github.com/soupdiver/creg/docker"
	"github.com/soupdiver/creg/eventmultiplexer"
)

var (
	fAddress       = flag.String("address", "", "Address to use for consul services")
	fConsulAddress = flag.String("consul", "", "Address of consul agent")
	fEtcdAddress   = flag.String("etcd", "", "Address of etcd agent")
	fHelp          = flag.BoolP("help", "h", false, "Print usage")
	fLabels        = flag.StringSliceP("labels", "l", []string{}, "Labels to append tp consul services")
	fSync          = flag.Bool("sync", false, "Sync consul services on start")
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
	// Handle flags
	flag.Parse()

	if *fHelp {
		flag.PrintDefaults()
		return nil
	}

	if fAddress == nil || *fAddress == "" {
		return fmt.Errorf("address is required")
	}

	// if fConsulAddress == nil || *fConsulAddress == "" {
	// 	return fmt.Errorf("consul address is required")
	// }

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

	// Get default config
	cfg := config.DefaultConfig

	// Overwrite settings as needed
	if *fConsulAddress != "" {
		cfg.ConsulConfig.Address = *fConsulAddress
	}
	if *fEtcdAddress != "" {
		cfg.EtcdConfig.Endpoints = append(cfg.EtcdConfig.Endpoints, *fEtcdAddress)
	}
	cfg.StaticLabels = []string{"dc=remote"}
	cfg.ForwardAddress = *fAddress

	// Setup Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}
	defer dockerClient.Close()

	// Setup event multiplexer
	multi := eventmultiplexer.New(docker.GetEventsForCreg(ctx, dockerClient, "creg"))
	multi.Run()

	// Setup Backends
	var enabledBackends []backends.Backend

	// if *fConsulAddress != "" {
	enabledBackends = append(enabledBackends, ConsulFromConfig(cfg))
	log.Print(1)
	// }
	if *fEtcdAddress != "" {
		enabledBackends = append(enabledBackends, EtcdFromConfig(cfg))
	}

	// Get currently running containers that we should register
	containers, err := docker.GetContainersForCreg(ctx, dockerClient, "creg")
	if err != nil {
		return fmt.Errorf("could not get creg containers: %w", err)
	}

	// Start backends
	var wg sync.WaitGroup
	for _, backend := range enabledBackends {
		log.Printf("Starting backend with %d containers", len(containers))
		wg.Add(1)
		go func(backend backends.Backend) {
			defer wg.Done()
			err := backend.Run(ctx, multi.NewOutput(), *fSync, containers)
			if err != nil {
				log.Printf("Backend failed: %s", err)
			}
		}(backend)
	}

	SetupSignalHandler(cancel)

	// Wait for backends to finish
	wg.Wait()

	log.Printf("Exit")

	return nil
}

func SetupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 5)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c

		log.Printf("Received %s, purging and cancelling context", s)

		// var err error
		// for _, backend := range enabledBackends {
		// 	err = backend.Purge()
		// 	if err != nil {
		// 		log.Printf("Could not purge backend: %s", err)
		// 	}
		// }

		cancel()
	}()
}

func ConsulFromConfig(cfg config.Config) *consul.Backend {
	consulBackend, err := consul.New(cfg.ConsulConfig, consul.WithStaticLabels([]string{"dc=remote"}))
	if err != nil {
		log.Fatalf("could not create consul backend: %s", err)
	}
	consulBackend.ForwardAddress = cfg.ForwardAddress

	return consulBackend
}

func EtcdFromConfig(cfg config.Config) *etcd.Backend {
	c, err := etcd.New(*cfg.EtcdConfig, etcd.WithStaticLabels(cfg.StaticLabels))
	if err != nil {
		log.Fatalf("could not create etcd backend: %s", err)
	}

	return c
}
