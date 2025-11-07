package middleware

import (
	"net/http"
	"strings"
)

// CORS returns middleware that adds CORS headers to responses.
// The allowedOrigins parameter specifies which origins are permitted to make cross-origin requests.
// If allowedOrigins is empty, CORS headers are not added (CORS is effectively disabled).
//
// The middleware:
//   - Sets Access-Control-Allow-Origin to the request's Origin if it's in the allowed list
//   - Sets Access-Control-Allow-Methods to allow GET, POST, OPTIONS
//   - Sets Access-Control-Allow-Headers to allow Content-Type and Authorization
//   - Sets Access-Control-Max-Age to 86400 (24 hours) for preflight caching
//   - Handles preflight OPTIONS requests by returning 200 OK for allowed origins, 403 for disallowed
//
// Note on Access-Control-Allow-Credentials:
// This header is not currently set because credentials (cookies, authorization headers) are not required
// for the current API. If your application needs to support credentials in cross-origin requests,
// uncomment the line that sets this header to "true". When credentials are allowed, the
// Access-Control-Allow-Origin cannot be "*" and must be a specific origin.
//
// Example usage:
//
//	allowedOrigins := []string{"http://localhost:5173", "https://example.com"}
//	handler := CORS(allowedOrigins)(myHandler)
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CORS headers if no origins are configured
			if len(allowedOrigins) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")
			originAllowed := origin != "" && isAllowedOrigin(origin, allowedOrigins)

			// Set CORS headers if the origin is allowed
			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
				// Uncomment the next line if your API needs to support credentials:
				// w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight OPTIONS requests
			if r.Method == http.MethodOptions {
				if originAllowed {
					w.WriteHeader(http.StatusOK)
				} else {
					// Return 403 Forbidden for OPTIONS requests from disallowed origins
					w.WriteHeader(http.StatusForbidden)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isAllowedOrigin checks if the given origin is in the allowed origins list.
// The comparison is case-sensitive and must match exactly.
func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	// Normalize origin by removing trailing slash
	origin = strings.TrimSuffix(origin, "/")

	for _, allowed := range allowedOrigins {
		// Normalize allowed origin by removing trailing slash
		allowed = strings.TrimSuffix(allowed, "/")
		if origin == allowed {
			return true
		}
	}
	return false
}
