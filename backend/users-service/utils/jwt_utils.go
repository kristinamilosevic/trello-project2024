package utils

import (
	"fmt"
	"os"
	"time"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru

	"github.com/golang-jwt/jwt/v5"
)

// Učitaj tajni ključ iz okruženja
var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(email, role string) (string, error) {
	logging.Logger.Debugf("Event ID: GENERATE_TOKEN_START, Description: Attempting to generate JWT token for email: %s, role: %s", email, role)

	if len(jwtSecret) == 0 {
		logging.Logger.Errorf("Event ID: GENERATE_TOKEN_SECRET_MISSING, Description: JWT_SECRET environment variable is not set or is empty.")
		return "", fmt.Errorf("JWT_SECRET is not set")
	}

	claims := &Claims{
		Email: email,
		Role:  role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtSecret)
	if err != nil {
		logging.Logger.Errorf("Event ID: GENERATE_TOKEN_SIGNING_FAILED, Description: Failed to sign JWT token for email '%s': %v", email, err)
		return "", err
	}

	logging.Logger.Infof("Event ID: GENERATE_TOKEN_SUCCESS, Description: Successfully generated JWT token for email: %s", email)
	return signedToken, nil
}

func GetEmailFromToken(tokenStr string) (string, error) {
	logging.Logger.Debug("Event ID: GET_EMAIL_FROM_TOKEN_START, Description: Attempting to extract email from token.")
	claims, err := parseToken(tokenStr)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_EMAIL_FROM_TOKEN_PARSE_FAILED, Description: Failed to parse token when getting email: %v", err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GET_EMAIL_FROM_TOKEN_SUCCESS, Description: Successfully extracted email '%s' from token.", claims.Email)
	return claims.Email, nil
}

func ValidateToken(tokenStr string) (*Claims, error) {
	logging.Logger.Debug("Event ID: VALIDATE_TOKEN_START, Description: Attempting to validate JWT token.")
	claims, err := parseToken(tokenStr)
	if err != nil {
		logging.Logger.Warnf("Event ID: VALIDATE_TOKEN_PARSE_FAILED, Description: Failed to parse token during validation: %v", err)
		return nil, err
	}
	if claims.ExpiresAt.Before(time.Now()) {
		logging.Logger.Warnf("Event ID: VALIDATE_TOKEN_EXPIRED, Description: Token for email '%s' has expired.", claims.Email)
		return nil, fmt.Errorf("token has expired")
	}
	logging.Logger.Infof("Event ID: VALIDATE_TOKEN_SUCCESS, Description: JWT token for email '%s' is valid.", claims.Email)
	return claims, nil
}

func parseToken(tokenStr string) (*Claims, error) {
	logging.Logger.Debug("Event ID: PARSE_TOKEN_START, Description: Attempting to parse raw JWT token string.")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logging.Logger.Warnf("Event ID: PARSE_TOKEN_UNEXPECTED_METHOD, Description: Unexpected signing method for token: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		logging.Logger.Warnf("Event ID: PARSE_TOKEN_INVALID, Description: Invalid token or parsing failed: %v", err)
		return nil, fmt.Errorf("invalid token")
	}
	logging.Logger.Debug("Event ID: PARSE_TOKEN_SUCCESS, Description: Successfully parsed JWT token.")
	return claims, nil
}
