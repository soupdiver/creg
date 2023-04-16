FROM alpine:3

COPY ./creg_linux_amd64_v1/creg /creg

CMD ["/creg"]
