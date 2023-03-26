build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -v -o ./bin/creg ./main.go

image-docker: build
	docker build -t soupdiver/creg:latest .

clean:
	rm -rf ./bin
