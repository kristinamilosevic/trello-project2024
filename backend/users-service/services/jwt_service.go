package services

import (
	"fmt"
	"os"
	"time"

	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru

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
	logging.Logger.Debug("Event ID: NEW_JWT_SERVICE, Description: Initializing JWTService.")
	return &JWTService{secretKey: secretKey}
}

// GenerateEmailVerificationToken kreira JWT token za verifikaciju email-a
func (s *JWTService) GenerateEmailVerificationToken(username string) (string, error) {
	logging.Logger.Debugf("Event ID: GENERATE_EMAIL_VERIFICATION_TOKEN_START, Description: Generating email verification token for username: %s", username)
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		logging.Logger.Errorf("Event ID: GENERATE_EMAIL_VERIFICATION_TOKEN_SIGNING_FAILED, Description: Failed to sign email verification token for username '%s': %v", username, err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GENERATE_EMAIL_VERIFICATION_TOKEN_SUCCESS, Description: Email verification token generated for username: %s", username)
	return signedToken, nil
}

func (s *JWTService) VerifyEmailVerificationToken(tokenString string) (string, error) {
	logging.Logger.Debug("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_START, Description: Verifying email verification token.")
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logging.Logger.Warnf("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_UNEXPECTED_METHOD, Description: Unexpected signing method for email verification token: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		logging.Logger.Warnf("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_PARSE_FAILED, Description: Failed to parse email verification token: %v", err)
		return "", err
	}

	// Proveri da li su claim-ovi validni i izvuci email
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		email, emailOk := claims["email"].(string)
		if !emailOk {
			logging.Logger.Warn("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_EMAIL_CLAIM_MISSING, Description: 'email' claim missing or not a string in verification token.")
			return "", fmt.Errorf("email claim missing or invalid")
		}
		logging.Logger.Infof("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_SUCCESS, Description: Email verification token verified for email: %s", email)
		return email, nil
	}

	logging.Logger.Warn("Event ID: VERIFY_EMAIL_VERIFICATION_TOKEN_INVALID, Description: Invalid email verification token.")
	return "", fmt.Errorf("invalid token")
}

// GenerateAuthToken kreira JWT token za autentifikaciju korisnika
func (s *JWTService) GenerateAuthToken(username, role string) (string, error) {
	logging.Logger.Debugf("Event ID: GENERATE_AUTH_TOKEN_START, Description: Generating authentication token for username: %s, role: %s", username, role)
	claims := Claims{
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		logging.Logger.Errorf("Event ID: GENERATE_AUTH_TOKEN_SIGNING_FAILED, Description: Failed to sign authentication token for username '%s': %v", username, err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GENERATE_AUTH_TOKEN_SUCCESS, Description: Authentication token generated for username: %s", username)
	return signedToken, nil
}

// ValidateToken proverava validnost JWT tokena
func (s *JWTService) ValidateToken(tokenStr string) (*Claims, error) {
	logging.Logger.Debug("Event ID: VALIDATE_TOKEN_START, Description: Validating JWT token.")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Proveri da li je algoritam potpisivanja ispravan
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logging.Logger.Warnf("Event ID: VALIDATE_TOKEN_UNEXPECTED_METHOD, Description: Unexpected signing method: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})
	if err != nil || !token.Valid {
		logging.Logger.Warnf("Event ID: VALIDATE_TOKEN_FAILED, Description: JWT token validation failed: %v", err)
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		logging.Logger.Warn("Event ID: VALIDATE_TOKEN_CLAIMS_INVALID, Description: Invalid claims format in JWT token.")
		return nil, fmt.Errorf("invalid token claims")
	}
	logging.Logger.Infof("Event ID: VALIDATE_TOKEN_SUCCESS, Description: JWT token validated successfully for username: %s", claims.Username)
	return claims, nil
}

// GenerateMagicLinkToken kreira JWT token za magic link prijavu
func (s *JWTService) GenerateMagicLinkToken(username string, role string) (string, error) {
	logging.Logger.Debugf("Event ID: GENERATE_MAGIC_LINK_TOKEN_START, Description: Generating magic link token for username: %s, role: %s", username, role)
	claims := Claims{
		Username: username,
		Role:     role,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		logging.Logger.Errorf("Event ID: GENERATE_MAGIC_LINK_TOKEN_SIGNING_FAILED, Description: Failed to sign magic link token for username '%s': %v", username, err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GENERATE_MAGIC_LINK_TOKEN_SUCCESS, Description: Magic link token generated for username: %s", username)
	return signedToken, nil
}

func (s *JWTService) GeneratePasswordResetToken(username string) (string, error) {
	logging.Logger.Debugf("Event ID: GENERATE_PASSWORD_RESET_TOKEN_START, Description: Generating password reset token for username: %s", username)
	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		logging.Logger.Errorf("Event ID: GENERATE_PASSWORD_RESET_TOKEN_SIGNING_FAILED, Description: Failed to sign password reset token for username '%s': %v", username, err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GENERATE_PASSWORD_RESET_TOKEN_SUCCESS, Description: Password reset token generated for username: %s", username)
	return signedToken, nil
}

func (s *JWTService) ExtractRoleFromToken(tokenString string) (string, error) {
	logging.Logger.Debug("Event ID: EXTRACT_ROLE_FROM_TOKEN_START, Description: Attempting to extract role from token.")
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		logging.Logger.Errorf("Event ID: EXTRACT_ROLE_FROM_TOKEN_VALIDATION_FAILED, Description: Token validation failed when extracting role: %v", err)
		return "", err
	}

	// Role je već string, nema potrebe za type assertion-om
	role := claims.Role

	if role == "" {
		logging.Logger.Warn("Event ID: EXTRACT_ROLE_FROM_TOKEN_ROLE_MISSING, Description: Role not found in token!")
		return "", fmt.Errorf("role not found in token")
	}

	logging.Logger.Debugf("Event ID: EXTRACT_ROLE_FROM_TOKEN_SUCCESS, Description: Extracted Role from Token: %s", role)
	return role, nil
}

// ParseToken parsira JWT token i vraća claimove
func (s *JWTService) ParseToken(tokenString string) (jwt.MapClaims, error) {
	logging.Logger.Debug("Event ID: PARSE_TOKEN_START, Description: Parsing JWT token to retrieve claims.")
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Proveri da li je algoritam potpisivanja ispravan
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			logging.Logger.Warnf("Event ID: PARSE_TOKEN_UNEXPECTED_METHOD, Description: Unexpected signing method: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secretKey), nil
	})
	if err != nil {
		logging.Logger.Warnf("Event ID: PARSE_TOKEN_FAILED, Description: Failed to parse token: %v", err)
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		logging.Logger.Warn("Event ID: PARSE_TOKEN_INVALID_CLAIMS_OR_TOKEN, Description: Invalid claims or token not valid during parsing.")
		return nil, fmt.Errorf("invalid token")
	}
	logging.Logger.Debug("Event ID: PARSE_TOKEN_SUCCESS, Description: Successfully parsed JWT token and extracted claims.")
	return claims, nil
}
