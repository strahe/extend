package main

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestAuthVerify_WithValidToken(t *testing.T) {
	signingKey := []byte("secret")
	token, _ := authNew(signingKey, "testuser", time.Hour)

	user, err := authVerify(signingKey, token)

	assert.NoError(t, err)
	assert.Equal(t, "testuser", user)
}

func TestAuthVerify_WithInvalidToken(t *testing.T) {
	signingKey := []byte("secret")
	wrongKey := []byte("wrongsecret")
	token, _ := authNew(signingKey, "testuser", time.Hour)

	_, err := authVerify(wrongKey, token)

	assert.Error(t, err)
}

func TestAuthVerify_WithExpiredToken(t *testing.T) {
	signingKey := []byte("secret")
	token, _ := authNew(signingKey, "testuser", -time.Hour)

	_, err := authVerify(signingKey, token)

	assert.Error(t, err)
}

func TestAuthNew_WithValidCredentials(t *testing.T) {
	signingKey := []byte("secret")
	token, err := authNew(signingKey, "testuser", time.Hour)

	assert.NoError(t, err)

	_, err = jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return signingKey, nil
	})

	assert.NoError(t, err)
}
