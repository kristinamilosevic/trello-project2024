# 1. Faza: Build aplikacije
FROM golang:alpine as build_container

# Setovanje radnog direktorijuma
WORKDIR /app

# Kopiranje mod fajlova i preuzimanje zavisnosti
COPY go.mod . 
COPY go.sum .
RUN go mod download

# Kopiranje ostatka koda i build aplikacije
COPY . .
RUN go build -o users-service .

# 2. Faza: Runtime okruženje
FROM alpine

# Dodavanje potrebnih alata za mrežu i SSL
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Setovanje radnog direktorijuma
WORKDIR /app

# Kopiranje binarne aplikacije i .env fajla
COPY --from=build_container /app/users-service .
COPY --from=build_container /app/.env .

# Ekspozicija porta
EXPOSE 8001

# Dozvola za pokretanje aplikacije
RUN chmod +x /app/users-service

# Pokretanje aplikacije
ENTRYPOINT ["./users-service"]
