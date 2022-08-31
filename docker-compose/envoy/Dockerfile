FROM golang:1.18.4-alpine3.16

WORKDIR /src/ratelimit

COPY ./ ./

RUN export REDIS_SOCKET_TYPE=tcp
RUN export REDIS_URL=host.docker.internal:6379
RUN export LOG_LEVEL=debug
RUN export USE_STATSD=false
RUN export GRPC_PORT=10004
RUN export RUNTIME_ROOT=$PWD
RUN export RUNTIME_SUBDIRECTORY=rlconfig
RUN export RUNTIME_WATCH_ROOT=false

RUN go get github.com/envoyproxy/ratelimit/src/service_cmd

CMD go run github.com/envoyproxy/ratelimit/src/service_cmd


# ENTRYPOINT [ "" ] 

