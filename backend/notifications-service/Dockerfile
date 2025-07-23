FROM golang:alpine as build_container

# Setovanje radnog direktorijuma
WORKDIR /app

COPY go.mod . 
COPY go.sum . 
RUN go mod download 
COPY . .

RUN go build -o notifications-service .

FROM alpine
WORKDIR /usr/bin
COPY --from=build_container /app/notifications-service . 
RUN mkdir -p /app/logs && chown nobody:nogroup /app/logs
EXPOSE 8004
ENTRYPOINT ["notifications-service"]
