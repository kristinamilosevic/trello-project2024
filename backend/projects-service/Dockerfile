FROM golang:alpine as build_container

WORKDIR /app

COPY projects-service/go.mod projects-service/go.sum ./projects-service/
COPY utils/go.mod ./utils/

COPY projects-service ./projects-service
COPY utils ./utils

RUN cd projects-service && go mod download

RUN cd projects-service && go build -o ../projects-service .

FROM alpine
RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin
COPY --from=build_container /app/projects-service .
EXPOSE 8001
RUN chmod +x /usr/bin/projects-service
ENTRYPOINT ["./projects-service"]
