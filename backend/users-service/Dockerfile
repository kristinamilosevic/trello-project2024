FROM golang:alpine as build_container

WORKDIR /app

# Kopira go.mod i go.sum iz users-service i utils pre go mod download
COPY users-service/go.mod users-service/go.sum ./users-service/
COPY utils/go.mod ./utils/

# Copy ceo kod
COPY users-service ./users-service
COPY utils ./utils

# Pokreni go mod download u users-service folderu 
RUN cd users-service && go mod download

# Build aplikacije
RUN cd users-service && go build -o ../users-service .

FROM alpine
RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin
COPY --from=build_container /app/users-service .
EXPOSE 8001
RUN chmod +x /usr/bin/users-service
ENTRYPOINT ["./users-service"]
