
FROM golang:alpine as build_container

WORKDIR /app


COPY api-composer-service/go.mod api-composer-service/go.sum ./api-composer-service/
COPY api-composer-service ./api-composer-service



RUN cd api-composer-service && go mod download
RUN cd api-composer-service && go build -o ../api-composer-service


FROM alpine

RUN apk add --no-cache ca-certificates && update-ca-certificates

WORKDIR /usr/bin


COPY --from=build_container /app/api-composer-service .

EXPOSE 8006

ENTRYPOINT ["./api-composer-service"]
