.PHONY: build test
build:
	goreleaser --skip-publish --auto-snapshot --clean

release:
	goreleaser --rm-dist

image-docker: build
	docker build -t soupdiver/creg:latest .

image-docker-push: image-docker
	docker push soupdiver/creg:latest

image-docker-ci:
	docker build -t soupdiver/creg-ci:latest -f Dockerfile.ci .

image-docker-ci-push: image-docker-ci
	docker push soupdiver/creg-ci:latest

test: image-docker
	docker tag soupdiver/creg:latest soupdiver/creg:testing
	go test -v ./...

clean:
	rm -rf ./bin
