package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/config"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
)

const maximumAuthBody = 4 << 10

type Handler struct {
	service        *Service
	limiter        *RateLimiter
	config         config.AuthConfig
	logger         *slog.Logger
	allowedOrigins map[string]struct{}
}

func NewHandler(service *Service, cfg config.AuthConfig, logger *slog.Logger) *Handler {
	origins := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		origins[strings.TrimSuffix(origin, "/")] = struct{}{}
	}
	return &Handler{
		service: service, limiter: NewRateLimiter(cfg.RateLimit, cfg.RateWindow),
		config: cfg, logger: logger, allowedOrigins: origins,
	}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/auth/register", h.register)
	router.Post("/auth/login", h.login)
	router.Post("/auth/refresh", h.refresh)
	router.With(h.service.Middleware).Post("/auth/logout", h.logout)
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request credentialsRequest
	if !decodeRequest(w, r, maximumAuthBody, &request) {
		return
	}
	email, _ := normalizeEmail(request.Email)
	if !h.allow(w, r, "register:"+clientIP(r)) || !h.allow(w, r, "register:"+clientIP(r)+":"+email) {
		return
	}
	result, err := h.service.Register(r.Context(), RegisterInput(request), requestMetadata(r))
	if err != nil {
		h.writeServiceError(w, r, "register", err)
		return
	}
	h.setRefreshCookie(w, result)
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteData(w, r, http.StatusCreated, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request credentialsRequest
	if !decodeRequest(w, r, maximumAuthBody, &request) {
		return
	}
	email, _ := normalizeEmail(request.Email)
	if !h.allow(w, r, "login:"+clientIP(r)) || !h.allow(w, r, "login:"+clientIP(r)+":"+email) {
		return
	}
	result, err := h.service.Login(r.Context(), LoginInput(request), requestMetadata(r))
	if err != nil {
		h.writeServiceError(w, r, "login", err)
		return
	}
	h.setRefreshCookie(w, result)
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteData(w, r, http.StatusOK, result)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.validOrigin(r) {
		httpserver.WriteProblem(w, r, http.StatusForbidden, "invalid_origin", "Forbidden", "Request origin is not allowed")
		return
	}
	cookie, err := r.Cookie(h.config.CookieName)
	if err != nil || cookie.Value == "" {
		h.clearRefreshCookie(w)
		h.writeServiceError(w, r, "refresh", ErrInvalidRefresh)
		return
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	key := "refresh:" + clientIP(r) + ":" + hex.EncodeToString(digest[:8])
	if !h.allow(w, r, "refresh:"+clientIP(r)) || !h.allow(w, r, key) {
		return
	}
	result, err := h.service.Refresh(r.Context(), cookie.Value, requestMetadata(r))
	if err != nil {
		h.clearRefreshCookie(w)
		h.writeServiceError(w, r, "refresh", err)
		return
	}
	h.setRefreshCookie(w, result)
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteData(w, r, http.StatusOK, result)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.validOrigin(r) {
		httpserver.WriteProblem(w, r, http.StatusForbidden, "invalid_origin", "Forbidden", "Request origin is not allowed")
		return
	}
	if !h.allow(w, r, "logout:"+clientIP(r)) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	if cookie, err := r.Cookie(h.config.CookieName); err == nil && cookie.Value != "" {
		if err := h.service.Logout(r.Context(), principal.UserID, cookie.Value); err != nil {
			h.writeServiceError(w, r, "logout", err)
			return
		}
	}
	h.clearRefreshCookie(w)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, limit int64, destination any) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		httpserver.WriteProblem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported media type", "Content-Type must be application/json")
		return false
	}
	if err := httpserver.DecodeJSON(w, r, limit, destination); err != nil {
		status := http.StatusBadRequest
		code := "invalid_json"
		title := "Invalid JSON"
		detail := "Request body must contain one valid JSON object with only supported fields"
		if errors.Is(err, httpserver.ErrBodyTooLarge) {
			status, code, title, detail = http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large", "Request body exceeds the allowed size"
		}
		httpserver.WriteProblem(w, r, status, code, title, detail)
		return false
	}
	return true
}

func (h *Handler) allow(w http.ResponseWriter, r *http.Request, key string) bool {
	allowed, retry := h.limiter.Allow(key)
	if allowed {
		return true
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	httpserver.WriteProblem(w, r, http.StatusTooManyRequests, "rate_limited", "Too many requests", "Try again later")
	return false
}

func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		httpserver.WriteProblem(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", "Email or password does not satisfy the documented constraints")
	case errors.Is(err, ErrEmailAlreadyExists):
		httpserver.WriteProblem(w, r, http.StatusConflict, "email_already_exists", "Conflict", "An account with this email already exists")
	case errors.Is(err, ErrInvalidCredentials):
		httpserver.WriteProblem(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials", "Email or password is incorrect")
	case errors.Is(err, ErrInvalidRefresh):
		httpserver.WriteProblem(w, r, http.StatusUnauthorized, "invalid_refresh_token", "Unauthorized", "Refresh token is invalid or expired")
	default:
		h.logger.ErrorContext(r.Context(), "auth operation failed", slog.String("operation", operation))
		httpserver.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", "Request could not be processed")
	}
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, result AuthResult) {
	maxAge := int(math.Ceil(result.RefreshExpires.Sub(h.service.now().UTC()).Seconds()))
	http.SetCookie(w, &http.Cookie{
		Name: h.config.CookieName, Value: result.RefreshToken, Path: "/api/v1/auth",
		Expires: result.RefreshExpires.UTC(), MaxAge: maxAge, HttpOnly: true,
		Secure: h.config.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: h.config.CookieName, Value: "", Path: "/api/v1/auth",
		MaxAge: -1, HttpOnly: true, Secure: h.config.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) validOrigin(r *http.Request) bool {
	source := strings.TrimSpace(r.Header.Get("Origin"))
	if source == "" {
		referer, err := url.Parse(r.Referer())
		if err != nil || referer.Scheme == "" || referer.Host == "" {
			return false
		}
		source = referer.Scheme + "://" + referer.Host
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil ||
		parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	normalized := parsed.Scheme + "://" + parsed.Host
	_, allowed := h.allowedOrigins[normalized]
	return allowed
}

func requestMetadata(r *http.Request) RequestMetadata {
	ipValue := clientIP(r)
	var ip *string
	if net.ParseIP(ipValue) != nil {
		ip = &ipValue
	}
	userAgentValue := strings.TrimSpace(r.UserAgent())
	if len(userAgentValue) > 1000 {
		userAgentValue = userAgentValue[:1000]
	}
	var userAgent *string
	if userAgentValue != "" {
		userAgent = &userAgentValue
	}
	return RequestMetadata{IP: ip, UserAgent: userAgent}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	if net.ParseIP(r.RemoteAddr) != nil {
		return r.RemoteAddr
	}
	return "unknown"
}
