package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

func authVerify(signingKey []byte, tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		sub, err := claims.GetSubject()
		if err != nil {
			return "", err
		}
		return sub, nil
	}
	return "", fmt.Errorf("unexpected claims type: %T", token.Claims)
}

func authNew(signingKey []byte, username string, duration time.Duration) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   username,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}
	if duration != 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(duration))
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func authMiddleware(secret []byte) mux.MiddlewareFunc {
	authHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			bearerToken := strings.Split(authHeader, " ")
			if len(bearerToken) != 2 {
				log.Warnf("missing Bearer prefix in auth header: %s", authHeader)
				warpResponse(w, http.StatusUnauthorized, nil, fmt.Errorf("invalid token"))
				return
			}
			user, err := authVerify(secret, bearerToken[1])
			if err != nil {
				log.Warnf("JWT Verification failed (originating from %s): %s", r.RemoteAddr, err)
				warpResponse(w, http.StatusUnauthorized, nil, fmt.Errorf("invalid token"))
				return
			}
			log.Infow("requests", "user", user, "method", r.Method, "path", r.URL.Path)
			ctx := WithAuthUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
	if secret == nil {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}
	return authHandler
}

type userCtxKey struct{}

func WithAuthUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userCtxKey{}, user)
}
