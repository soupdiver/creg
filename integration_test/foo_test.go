package integratontest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMain(m *testing.M) {
	err := setUp("docker-compose-consul.yml")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := tearDown("docker-compose-consul.yml")
		if err != nil {
			panic(err)
		}
	}()

	// Wait for services to be up and running
	time.Sleep(5 * time.Second)

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func setUp(composeFile string) error {
	cmd := exec.Command("docker-compose", "-f", composeFile, "up", "-d")
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "could not set up: %s", composeFile)
	}

	return nil
}

func tearDown(composeFile string) error {
	cmd := exec.Command("docker-compose", "-f", composeFile, "down")
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "could not tear down: %s", composeFile)
	}

	return nil
}

func isConsulServiceRegistered(serviceName string) (bool, error) {
	resp, err := http.Get("http://localhost:8500/v1/catalog/service/" + serviceName)
	if err != nil {
		return false, errors.Wrapf(err, "could not get consul service %s", serviceName)
	}
	defer resp.Body.Close()

	var services []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&services)
	if err != nil {
		return false, errors.Wrap(err, "could not decode body")
	}

	return len(services) > 0, nil
}

func TestConsulServiceRegistrationWithoutLabels(t *testing.T) {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image: "nginx",
		// ExposedPorts: []string{"80/tcp"}, // Adjust this according to your service
		WaitingFor: wait.ForHTTP("/"), // Adjust the wait strategy as needed
	}
	myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %s", err)
	}
	defer myservice.Terminate(ctx)

	// Allow some time for registration
	time.Sleep(5 * time.Second)

	registered, err := isConsulServiceRegistered("myservice")
	if err != nil {
		t.Fatalf("Failed to check if service is registered: %s", err)
	}

	if registered {
		t.Errorf("Expected service to not be registered, but it was")
	}
}
