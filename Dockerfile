FROM alpine:3

COPY ./bin/creg /creg

CMD ["/creg"]
