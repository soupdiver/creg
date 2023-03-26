FROM alpine

COPY ./bin/creg /creg

CMD ["/creg"]
