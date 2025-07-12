# Build stage
FROM golang:alpine as build_container

WORKDIR /app

# Kopiraj go.mod i go.sum iz workflow-service i utils pre go mod download
COPY workflow-service/go.mod workflow-service/go.sum ./workflow-service/
COPY utils/go.mod ./utils/

# Kopiraj ceo kod
COPY workflow-service ./workflow-service
COPY utils ./utils

# Pokreni go mod download u workflow-service folderu
RUN cd workflow-service && go mod download

# Build aplikacije
RUN cd workflow-service && go build -o ../workflow-service .

# Final stage
FROM alpine

RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin
COPY --from=build_container /app/workflow-service .

EXPOSE 8005

RUN chmod +x /usr/bin/workflow-service

ENTRYPOINT ["./workflow-service"]
