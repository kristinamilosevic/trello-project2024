package middleware

import (
	"net/http"
	"strings"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
	"trello-project/microservices/users-service/utils"
)

func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Logger.Debugf("Event ID: JWT_AUTH_MIDDLEWARE_START, Description: Starting JWTAuthMiddleware for request to %s %s", r.Method, r.URL.Path)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logging.Logger.Warnf("Event ID: JWT_AUTH_MISSING_HEADER, Description: Authorization header missing for request to %s %s", r.Method, r.URL.Path)
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader { // If "Bearer " prefix was not present
			logging.Logger.Warnf("Event ID: JWT_AUTH_BEARER_PREFIX_MISSING, Description: Bearer prefix missing in Authorization header for request to %s %s", r.Method, r.URL.Path)
		}
		logging.Logger.Debugf("Event ID: JWT_AUTH_TOKEN_EXTRACTED, Description: Extracted token (truncated) for request to %s %s: %s...", r.Method, r.URL.Path, tokenStr[:min(len(tokenStr), 10)])

		_, err := utils.ValidateToken(tokenStr)
		if err != nil {
			logging.Logger.Warnf("Event ID: JWT_AUTH_INVALID_TOKEN, Description: Invalid token provided for request to %s %s: %v", r.Method, r.URL.Path, err)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		logging.Logger.Debugf("Event ID: JWT_AUTH_SUCCESS, Description: Token validated successfully for request to %s %s. Proceeding to next handler.", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// Helper function to find the minimum of two integers (Go 1.20+ has slices.Min, but for broader compatibility)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
