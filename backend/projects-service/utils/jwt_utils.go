package utils

import (
	"fmt"
	"os"
	"trello-project/microservices/projects-service/logging"

	"github.com/dgrijalva/jwt-go"
)

// ekstrakcija username iz tokena
func ExtractManagerUsernameFromToken(tokenString string) (string, error) {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		logging.Logger.Warn("JWT_SECRET is not set in environment variables.")
		return "", fmt.Errorf("JWT_SECRET is not set")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	if err != nil {
		logging.Logger.Errorf("Error parsing token: %v", err)
		return "", fmt.Errorf("error parsing token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		username, exists := claims["username"]
		if !exists {
			logging.Logger.Errorf("Username claim not found in token. Claims: %+v", claims)
			return "", fmt.Errorf("username claim not found in token")
		}

		return username.(string), nil
	}
	logging.Logger.Warn("Invalid token provided or token is not valid.")
	return "", fmt.Errorf("invalid token")
}
