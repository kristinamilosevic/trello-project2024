FROM golang:alpine as build_container

WORKDIR /app

COPY tasks-service/go.mod tasks-service/go.sum ./tasks-service/
COPY utils/go.mod ./utils/

COPY tasks-service ./tasks-service
COPY utils ./utils


RUN cd tasks-service && go mod download

RUN cd tasks-service && go build -o ../tasks-service .

FROM alpine
RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin
COPY --from=build_container /app/tasks-service .
EXPOSE 8001
RUN chmod +x /usr/bin/tasks-service
ENTRYPOINT ["./tasks-service"]
