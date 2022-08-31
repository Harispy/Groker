FROM golang:1.18.4-alpine3.16

WORKDIR /src/client

COPY ../ ./

RUN go build client.go

ENTRYPOINT ./client