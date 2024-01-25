build:
	goreleaser build --snapshot --clean

release:
	goreleaser --clean

image-docker: build
	docker build -t soupdiver/creg:latest .

image-docker-push: image-docker
	docker push soupdiver/creg:latest

image-docker-ci:
	docker build -t soupdiver/creg-ci:latest -f Dockerfile.ci .

image-docker-ci-push: image-docker-ci
	docker push soupdiver/creg-ci:latest

test: image-docker
	# Unit tests
	go test $(go list ./... | grep -v /integration_test/)
	
	@docker tag soupdiver/creg:latest soupdiver/creg:testing
	# Integration tests not cached
	go test -v -count=1 ./integration_test/...

test-integration: build
	cd integration_test && go test -v creg_test.go

clean:
	rm -rf ./dist

.PHONY: build release image-docker image-docker-push image-docker-ci image-docker-ci-push test clean
