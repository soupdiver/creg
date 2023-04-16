.PHONY: build test
build:
	goreleaser build --snapshot --rm-dist

release:
	goreleaser --clean

image-docker:
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

clean:
	rm -rf ./dist
