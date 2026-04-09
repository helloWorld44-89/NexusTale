package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// APIKeyResponse is the public representation of a stored API key.
// The raw key is never returned after creation.
type APIKeyResponse struct {
	ID        uuid.UUID `json:"id"`
	Provider  string    `json:"provider"`
	KeyHint   string    `json:"key_hint"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertAPIKey encrypts and stores the given plaintext key for the provider.
// If a key for that provider already exists it is replaced.
func (s *Service) UpsertAPIKey(ctx context.Context, userID uuid.UUID, provider, plaintext string) (*APIKeyResponse, error) {
	if len(plaintext) < 4 {
		return nil, apperror.Validation("api key is too short")
	}

	encrypted, err := s.encryptKey(plaintext)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("encrypt api key: %v", err))
	}

	hint := plaintext[len(plaintext)-4:]
	row, err := s.queries.UpsertAPIKey(ctx, sqlcgen.UpsertAPIKeyParams{
		UserID:       userID,
		Provider:     provider,
		EncryptedKey: encrypted,
		KeyHint:      hint,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("upsert api key: %v", err))
	}

	return toAPIKeyResponse(row), nil
}

// ListAPIKeys returns all stored key hints for the user (no raw keys).
func (s *Service) ListAPIKeys(ctx context.Context, userID uuid.UUID) ([]APIKeyResponse, error) {
	rows, err := s.queries.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list api keys: %v", err))
	}
	result := make([]APIKeyResponse, len(rows))
	for i, r := range rows {
		result[i] = *toAPIKeyResponse(r)
	}
	return result, nil
}

// DeleteAPIKey removes the stored key for the given provider.
func (s *Service) DeleteAPIKey(ctx context.Context, userID uuid.UUID, provider string) error {
	// Verify it exists before deleting so we can return a 404 if not found.
	_, err := s.queries.GetAPIKey(ctx, sqlcgen.GetAPIKeyParams{UserID: userID, Provider: provider})
	if err == pgx.ErrNoRows {
		return apperror.NotFound("api key", provider)
	}
	if err != nil {
		return apperror.Internal(fmt.Sprintf("get api key: %v", err))
	}
	return s.queries.DeleteAPIKey(ctx, sqlcgen.DeleteAPIKeyParams{UserID: userID, Provider: provider})
}

// DecryptAPIKey retrieves and decrypts the key for the given provider.
// Used internally by the AI adapter layer — never sent to clients.
func (s *Service) DecryptAPIKey(ctx context.Context, userID uuid.UUID, provider string) (string, error) {
	row, err := s.queries.GetAPIKey(ctx, sqlcgen.GetAPIKeyParams{UserID: userID, Provider: provider})
	if err == pgx.ErrNoRows {
		return "", apperror.NotFound("api key", provider)
	}
	if err != nil {
		return "", apperror.Internal(fmt.Sprintf("get api key: %v", err))
	}

	plaintext, err := s.decryptKey(row.EncryptedKey)
	if err != nil {
		return "", apperror.Internal(fmt.Sprintf("decrypt api key: %v", err))
	}
	return plaintext, nil
}

// ── encryption helpers ────────────────────────────────────────────────────────

// encryptKey encrypts plaintext using AES-256-GCM with the server encryption
// key from config. The 12-byte nonce is prepended to the ciphertext.
func (s *Service) encryptKey(plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, nil
}

// decryptKey reverses encryptKey.
func (s *Service) decryptKey(data []byte) (string, error) {
	block, err := aes.NewCipher(s.encKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// ParseEncryptionKey decodes a hex-encoded 32-byte key from config.
func ParseEncryptionKey(hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("encryption key must be hex-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes (64 hex chars), got %d", len(key))
	}
	return key, nil
}

func toAPIKeyResponse(r sqlcgen.UserApiKey) *APIKeyResponse {
	return &APIKeyResponse{
		ID:        r.ID,
		Provider:  r.Provider,
		KeyHint:   r.KeyHint,
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}
