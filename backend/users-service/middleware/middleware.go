package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"trello-project/microservices/users-service/utils"
)

// Mapiraj dozvole za svaku ulogu
var rolePermissions = map[string][]string{
	"manager": {"GET", "POST", "DELETE"},
	"member":  {"GET", "POST", "DELETE"},
}

// Middleware koji proverava JWT token i role
func JWTAuthMiddleware(next http.Handler, requiredRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dohvati Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header missing", http.StatusUnauthorized)
			return
		}

		// Izvuci token iz header-a
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := utils.ValidateToken(tokenStr)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Proveri da li korisnik ima potrebnu ulogu za pristup resursu
		role := claims.Role
		allowedMethods, exists := rolePermissions[role]
		if !exists {
			http.Error(w, "Role not recognized", http.StatusForbidden)
			return
		}

		// Proveri da li trenutni metod zahteva ima dozvolu za tu ulogu
		method := r.Method
		if !contains(allowedMethods, method) {
			http.Error(w, fmt.Sprintf("Role %s is not allowed to perform %s action", role, method), http.StatusForbidden)
			return
		}

		// Ako je sve u redu, prosledi zahtev dalje
		next.ServeHTTP(w, r)
	})
}

// Helper funkcija za proveru da li slice sadrži određeni string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
