module trello-project/microservices/users-service

go 1.22.0

require github.com/golang-jwt/jwt/v5 v5.2.1

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gorilla/mux v1.8.1
	github.com/joho/godotenv v1.5.1
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.mongodb.org/mongo-driver v1.17.1
	golang.org/x/crypto v0.29.0
	golang.org/x/exp v0.0.0-20241108190413-2d47ceb2692f
	golang.org/x/sync v0.9.0 // indirect
	golang.org/x/text v0.20.0 // indirect
)

require (
	github.com/google/uuid v1.6.0
	github.com/sirupsen/logrus v1.9.3
	github.com/sony/gobreaker v1.0.0
	trello-project/backend/utils v0.0.0
)

require (
	golang.org/x/sys v0.27.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

replace trello-project/backend/utils => ../utils
