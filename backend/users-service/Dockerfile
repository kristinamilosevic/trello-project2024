# Dockerfile za users servis
FROM golang:alpine as build_container

# Setovanje radnog direktorijuma
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o users-service .
RUN ls /app

FROM alpine
WORKDIR /usr/bin
COPY --from=build_container /app/users-service .
EXPOSE 8000
RUN chmod +x /usr/bin/users-service 
ENTRYPOINT ["users-service"]

