build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -v -ldflags "-s -w" -o ./bin/creg ./main.go

image-docker: build
	docker build -t soupdiver/creg:latest .

image-docker-push: image-docker
	docker push soupdiver/creg:latest

clean:
	rm -rf ./bin
