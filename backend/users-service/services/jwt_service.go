package services

import (
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
