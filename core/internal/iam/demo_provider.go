package iam

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type DemoProvider struct{}

func NewDemoProvider() *DemoProvider {
	return &DemoProvider{}
}

func (p *DemoProvider) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Always populate a super user
			u := &User{
				ID:       "demo-user",
				Username: "demo",
				Email:    "demo@druppie.ai",
				Groups:   []string{"admin", "root"}, // All powerful
			}
			ctx := context.WithValue(r.Context(), userContextKey, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (p *DemoProvider) GetUser(r *http.Request) (*User, error) {
	u, ok := r.Context().Value(userContextKey).(*User)
	if !ok {
		// Fallback for requests that somehow bypassed middleware
		return &User{
			ID:       "demo-user",
			Username: "demo",
			Email:    "demo@druppie.ai",
			Groups:   []string{"admin", "root"},
		}, nil
	}
	return u, nil
}

func (p *DemoProvider) RegisterRoutes(r chi.Router) {
	// Dummy login endpoint for UI compatibility
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token: "demo-token",
			User: User{
				ID:       "demo-user",
				Username: "demo",
				Email:    "demo@druppie.ai",
				Groups:   []string{"admin", "root"},
			},
		})
	})
	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
