package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"

	"github.com/soupdiver/creg/backends"
	adguardhomebackend "github.com/soupdiver/creg/backends/adguardhome"
	"github.com/soupdiver/creg/backends/consul"
	"github.com/soupdiver/creg/backends/debug"
	"github.com/soupdiver/creg/backends/etcd"
	"github.com/soupdiver/creg/config"
	"github.com/soupdiver/creg/docker"
	"github.com/soupdiver/creg/eventmultiplexer"
	"github.com/soupdiver/creg/podman"
	"github.com/soupdiver/creg/types"
)

var logr = logrus.New()

var (
	fAddress         = flag.String("address", "", "Address to use for consul services")
	fConsulAddress   = flag.String("consul", "", "Address of consul agent")
	fEtcdAddress     = flag.String("etcd", "", "Address of etcd agent")
	fAdguardHome     = flag.String("adguardhome", "", "Address of adguardhome server")
	fAdguardHomeAuth = flag.String("adguardhomeauth", "", "Auth of adguardhome server")
	fHelp            = flag.BoolP("help", "h", false, "Print usage")
	fDebug           = flag.BoolP("debug", "d", false, "Debug log")
	fDebugCaller     = flag.BoolP("debugCaller", "g", false, "Debug caller log")
	fLabels          = flag.StringSliceP("labels", "l", []string{}, "Labels to append tp consul services")
	fLogColor        = flag.Bool("color", true, "Colorize log output")
	fSync            = flag.Bool("sync", false, "Sync consul services on start")
	fEnableLabel     = flag.String("enable", "creg", "label on which to enable creg")
	fID              = flag.String("id", "creg-default", "Instance ID")
)

var (
// DefaultLabels = []string{"dc=remote"}
// CREDLabel     = "consul-reg.port"
)

func main() {
	if err := Run(); err != nil {
		logr.Fatalf("Fatal: %s", err)
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

	// Logging defaults
	logr.Out = os.Stdout
	logr.SetLevel(logrus.InfoLevel)
	logr.SetFormatter(&logrus.TextFormatter{})
	// if !*fLogColor {
	// 	logr.SetFormatter(&logrus.TextFormatter{
	// 		DisableColors: true,
	// 	})
	// }

	if fDebug != nil && *fDebug {
		logr.SetLevel(logrus.DebugLevel)
	}

	if fDebugCaller != nil && *fDebugCaller {
		logr.SetReportCaller(true)
	}

	log := logr.WithFields(logrus.Fields{"id": *fID})

	log.WithField("debug", *fDebug).Infof("Starting")

	ctx = context.WithValue(ctx, "log", log)

	// Get default config
	cfg := config.DefaultConfig

	// Overwrite settings as needed
	cfg.ID = *fID
	if *fConsulAddress != "" {
		cfg.ConsulConfig.Address = *fConsulAddress
	}
	if *fEtcdAddress != "" {
		cfg.EtcdConfig.Endpoints = append(cfg.EtcdConfig.Endpoints, *fEtcdAddress)
	}
	cfg.StaticLabels = *fLabels
	cfg.ForwardAddress = *fAddress

	// Setup Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}
	defer dockerClient.Close()

	podmanClient := podman.NewPodmanEventsClient("/run/podman/podman.sock")

	inputs := []<-chan types.ContainerEventV2{
		docker.GetEventsForCreg(ctx, dockerClient, *fEnableLabel, cfg.ID),
		// TODO: handle cregID
		podmanClient.GetEventsForCreg(ctx, *fEnableLabel, cfg.ID),
	}

	// Setup event multiplexer
	multi := eventmultiplexer.New(inputs...)
	multi.Run(ctx)

	// Setup Backends
	var enabledBackends []backends.Backend

	if *fDebug {
		log.WithField("label", *fEnableLabel).Printf("Enable debug")
		enabledBackends = append(enabledBackends, debug.New(ctx))
	}

	if *fConsulAddress != "" {
		log.WithField("label", *fEnableLabel).Printf("Enable consul")
		enabledBackends = append(enabledBackends, ConsulFromConfig(cfg, log))
		enabledBackends[len(enabledBackends)-1].(*consul.Backend).ServicePrefix = *fEnableLabel
	}

	if *fEtcdAddress != "" {
		log.Printf("Enable etcd")
		enabledBackends = append(enabledBackends, EtcdFromConfig(cfg, log))
	}

	if *fAdguardHome != "" {
		log.Printf("Enable adguardhome: %s", *fAdguardHomeAuth)
		b, err := adguardhomebackend.New(*fAdguardHome, *fAdguardHomeAuth)
		if err != nil {
			return fmt.Errorf("could not create adguardhome backend: %w", err)
		}
		enabledBackends = append(enabledBackends, b)
	}

	// Get currently running containers that we should register
	// containers, err := docker.GetContainersForCreg(ctx, dockerClient, *fEnableLabel)
	// if err != nil {
	// 	return fmt.Errorf("could not get creg containers: %w", err)
	// }

	// Start backends
	var wg sync.WaitGroup
	for _, backend := range enabledBackends {
		wg.Add(1)
		go func(backend backends.Backend) {
			defer wg.Done()
			err := backend.Run(ctx, multi.NewOutput(backend.GetName()), *fSync, []types.ContainerInfo{})
			if err != nil {
				log.Printf("Backend failed: %s", err)
			}
		}(backend)
	}

	SetupSignalHandler(cancel)

	// Wait for backends to finish
	wg.Wait()

	<-ctx.Done()

	log.Print("exit in 2s")
	time.Sleep(2 * time.Second)
	log.Print("exit")

	return nil
}

func SetupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 5)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c

		logr.Printf("Received %s, purging and cancelling context", s)

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

func ConsulFromConfig(cfg config.Config, log *logrus.Entry) *consul.Backend {
	consulBackend, err := consul.New(cfg.ConsulConfig,
		consul.WithStaticLabels(cfg.StaticLabels),
		consul.WithLogger(log),
		consul.WithID(cfg.ID),
	)
	if err != nil {
		log.Fatalf("could not create consul backend: %s", err)
	}
	consulBackend.ForwardAddress = cfg.ForwardAddress

	return consulBackend
}

func EtcdFromConfig(cfg config.Config, log *logrus.Entry) *etcd.Backend {
	c, err := etcd.New(*cfg.EtcdConfig,
		etcd.WithStaticLabels(cfg.StaticLabels),
		etcd.WithLogger(log),
	)
	if err != nil {
		log.Fatalf("could not create etcd backend: %s", err)
	}
	c.ID = cfg.ID

	return c
}
