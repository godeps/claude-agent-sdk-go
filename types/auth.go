package types

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// AuthProvider provides credentials for Claude API authentication.
type AuthProvider interface {
	Apply(opts *ClaudeAgentOptions) error
}

// APIKeyAuth provides a static API key.
type APIKeyAuth struct {
	Key string
}

func (a *APIKeyAuth) Apply(opts *ClaudeAgentOptions) error {
	if a.Key == "" {
		return fmt.Errorf("API key must not be empty")
	}
	opts.WithEnvVar("ANTHROPIC_API_KEY", a.Key)
	return nil
}

// BearerTokenAuth provides a bearer token.
type BearerTokenAuth struct {
	Token string
}

func (b *BearerTokenAuth) Apply(opts *ClaudeAgentOptions) error {
	if b.Token == "" {
		return fmt.Errorf("bearer token must not be empty")
	}
	opts.WithEnvVar("ANTHROPIC_AUTH_TOKEN", b.Token)
	return nil
}

// HMACAuth generates HMAC-signed auth credentials.
type HMACAuth struct {
	KeyID     string
	SecretKey string
}

func (h *HMACAuth) Apply(opts *ClaudeAgentOptions) error {
	if h.KeyID == "" || h.SecretKey == "" {
		return fmt.Errorf("HMAC key ID and secret key must not be empty")
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(h.SecretKey))
	mac.Write([]byte(timestamp))
	signature := hex.EncodeToString(mac.Sum(nil))

	opts.WithEnvVar("ANTHROPIC_AUTH_HMAC_KEY_ID", h.KeyID)
	opts.WithEnvVar("ANTHROPIC_AUTH_HMAC_SIGNATURE", signature)
	opts.WithEnvVar("ANTHROPIC_AUTH_HMAC_TIMESTAMP", timestamp)
	return nil
}

// NewAPIKeyAuth creates an API key auth provider.
func NewAPIKeyAuth(key string) AuthProvider {
	return &APIKeyAuth{Key: key}
}

// NewBearerTokenAuth creates a bearer token auth provider.
func NewBearerTokenAuth(token string) AuthProvider {
	return &BearerTokenAuth{Token: token}
}

// NewHMACAuth creates an HMAC auth provider.
func NewHMACAuth(keyID, secretKey string) AuthProvider {
	return &HMACAuth{KeyID: keyID, SecretKey: secretKey}
}
