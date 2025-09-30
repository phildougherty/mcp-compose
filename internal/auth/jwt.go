package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	ClientID string `json:"client_id"`
	UserID   string `json:"user_id,omitempty"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
}

func NewJWTManager(issuer string) (*JWTManager, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {

		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	return &JWTManager{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		issuer:     issuer,
	}, nil
}

func (j *JWTManager) GenerateAccessToken(clientID, userID, scope string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := JWTClaims{
		ClientID: clientID,
		UserID:   userID,
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{clientID},
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	return token.SignedString(j.privateKey)
}

func (j *JWTManager) ValidateAccessToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {

			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return j.publicKey, nil
	})

	if err != nil {

		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {

		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (j *JWTManager) GetPublicKey() *rsa.PublicKey {

	return j.publicKey
}