package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const cookieName = "parentald_session"

// cookieAuth wraps a handler with cookie-based authentication.
// Also accepts X-API-Key header as alternative (for HA integration).
// Redirects to /login if no valid auth is present (unless API key was attempted).
func cookieAuth(next http.Handler, secret, apiKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check API key first
		if apiKey != "" && r.Header.Get("X-API-Key") != "" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(apiKey)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "invalid API key", http.StatusUnauthorized)
			return
		}

		// Fall back to cookie auth
		if !validateSessionCookie(r, secret) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// createSessionCookie creates an HMAC-signed session cookie.
func createSessionCookie(username, secret string) *http.Cookie {
	expiry := time.Now().Add(10 * 365 * 24 * time.Hour) // ~10 years
	payload := fmt.Sprintf("%s|%d", username, expiry.Unix())
	sig := sign(payload, secret)
	value := base64.URLEncoding.EncodeToString([]byte(payload + "|" + sig))

	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

// clearSessionCookie returns a cookie that clears the session.
func clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
}

// validateSessionCookie checks if the request has a valid session cookie.
func validateSessionCookie(r *http.Request, secret string) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}

	decoded, err := base64.URLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), "|", 3)
	if len(parts) != 3 {
		return false
	}

	payload := parts[0] + "|" + parts[1]
	expectedSig := sign(payload, secret)

	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expectedSig)) != 1 {
		return false
	}

	// Check expiry
	var expiry int64
	fmt.Sscanf(parts[1], "%d", &expiry)
	if time.Now().Unix() > expiry {
		return false
	}

	return true
}

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}
