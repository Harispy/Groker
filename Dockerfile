FROM golang:1.18.4-alpine3.16

WORKDIR /src/broker

COPY ./ ./

RUN go build main.go

EXPOSE 8080

ENTRYPOINT ./main