package services

import (
	"fmt"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// JWTService sadrži funkcionalnost za rad sa JWT tokenima
type JWTService struct{}

// GenerateEmailVerificationToken kreira JWT token sa email adresom kao claim
func (s *JWTService) GenerateEmailVerificationToken(email string) (string, error) {
	// Postavljanje claim-ova za JWT token
	claims := jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Hour * 24).Unix(), // Token važi 24 sata
	}

	// Generisanje tokena sa HS256 algoritmom
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Potpisivanje tokena pomoću tajnog ključa
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
func (s *JWTService) VerifyEmailVerificationToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return "", err
	}

	// Proveri da li su claim-ovi validni i izvuci email
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		email := claims["email"].(string)
		return email, nil
	}

	return "", fmt.Errorf("invalid token")
}
