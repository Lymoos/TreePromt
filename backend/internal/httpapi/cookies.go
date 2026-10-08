package httpapi

import (
	"net/http"
	"strings"

	"prompttree/backend/internal/core"
)

// Браузерный клиент (заголовок X-PT-Client: web) получает refresh-токен только
// в httpOnly-cookie: скрипты страницы его не видят (архитектура п. 5).
const (
	webClientHeader   = "X-PT-Client"
	refreshCookie     = "pt_refresh"
	refreshCookiePath = "/api/v1/auth"
	// cookieMarker возвращается в JSON вместо токена: «токен в cookie».
	cookieMarker = "cookie"
)

func isWebClient(r *http.Request) bool { return r.Header.Get(webClientHeader) == "web" }

func (s *Server) secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return s.trustProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// deliverRefresh: для веба переносит refresh-токен из ответа в cookie.
func (s *Server) deliverRefresh(w http.ResponseWriter, r *http.Request, t core.Tokens) core.Tokens {
	if !isWebClient(r) {
		return t
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    t.RefreshToken,
		Path:     refreshCookiePath,
		MaxAge:   int(core.RefreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
	t.RefreshToken = cookieMarker
	return t
}

// refreshFromRequest: токен из тела запроса, а для веба — из cookie.
func (s *Server) refreshFromRequest(r *http.Request, body string) string {
	if body != "" && body != cookieMarker {
		return body
	}
	if c, err := r.Cookie(refreshCookie); err == nil {
		return c.Value
	}
	return ""
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	if !isWebClient(r) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Path:     refreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}
