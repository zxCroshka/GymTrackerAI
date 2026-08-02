package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/httpserver"
)

type principalContextKey struct{}

type Principal struct {
	UserID      string
	FamilyID    string
	AuthVersion int
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeUnauthorized(w, r)
			return
		}
		claims, err := s.tokens.parseAccess(parts[1])
		if err != nil {
			writeUnauthorized(w, r)
			return
		}
		state, err := s.repository.FindAuthStateByID(r.Context(), claims.Subject)
		if err != nil || state.Status != "active" || state.AuthVersion != claims.AuthVersion {
			writeUnauthorized(w, r)
			return
		}
		principal := Principal{UserID: state.ID, FamilyID: claims.FamilyID, AuthVersion: claims.AuthVersion}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
	})
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer realm=\"gymtracker\"")
	httpserver.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "Unauthorized", "Authentication is required")
}
