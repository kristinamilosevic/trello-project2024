FROM golang:alpine as build_container

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o utils .

FROM alpine
RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin
COPY --from=build_container /app/utils .

EXPOSE 8080
RUN chmod +x /usr/bin/utils
ENTRYPOINT ["./utils"]
