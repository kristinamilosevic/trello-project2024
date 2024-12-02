package services

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// JWTService struktura
type JWTService struct {
	secretKey string
}

// Claims struktura za JWT tokene
type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.StandardClaims
}

// Konstruktor za `JWTService`
func NewJWTService(secretKey string) *JWTService {
	return &JWTService{secretKey: secretKey}
}

// GenerateEmailVerificationToken kreira JWT token za verifikaciju email-a
func (s *JWTService) GenerateEmailVerificationToken(username string) (string, error) {
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
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

// GenerateAuthToken kreira JWT token za autentifikaciju korisnika
func (s *JWTService) GenerateAuthToken(username, role string) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), // Token važi 24 sata
			IssuedAt:  time.Now().Unix(),                     // Vreme izdavanja tokena
		},
	}

	// Generisanje tokena sa tajnim ključem iz environment promenljive
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return "", fmt.Errorf("missing JWT_SECRET environment variable")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("error signing token: %v", err)
	}

	log.Printf("Generated token for username=%s, role=%s", username, role)
	return signedToken, nil
}

func (s *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		return nil, fmt.Errorf("missing JWT_SECRET environment variable")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		log.Printf("Error parsing token: %v", err)
		return nil, fmt.Errorf("error parsing token: %v", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token or claims")
	}

	log.Printf("Validated token for username=%s, role=%s", claims.Username, claims.Role)
	return claims, nil
}

// GenerateMagicLinkToken kreira JWT token za magic link prijavu
func (s *JWTService) GenerateMagicLinkToken(username string, role string) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
