FROM alpine:3

COPY dist/creg_linux_amd64_v1/creg /creg

CMD ["/creg"]
