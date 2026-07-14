package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/storage"
)

type Service struct {
	queries            *sqlcgen.Queries
	jwtSecret          []byte
	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
	bcryptCost         int
	encKey             []byte // AES-256-GCM key for API key encryption
	storageClient      *storage.Client
}

// WithStorage wires a MinIO client so DeleteMe can clean up binary assets.
func (s *Service) WithStorage(sc *storage.Client) {
	s.storageClient = sc
}

func NewService(queries *sqlcgen.Queries, jwtSecret string, accessExpiry, refreshExpiry time.Duration, bcryptCost int, encKey []byte) *Service {
	return &Service{
		queries:            queries,
		jwtSecret:          []byte(jwtSecret),
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
		bcryptCost:         bcryptCost,
		encKey:             encKey,
	}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	// Check if user already exists
	_, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, apperror.Conflict("email already registered")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Internal(fmt.Sprintf("check user: %v", err))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("hash password: %v", err))
	}

	user, err := s.queries.CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: string(hash),
		Role:         sqlcgen.UserRoleAuthor,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create user: %v", err))
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, user.Email, user.DisplayName, Role(user.Role))
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User: UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        Role(user.Role),
			Plan:        user.Plan,
			CreatedAt:   user.CreatedAt.Time,
		},
		Tokens: *tokens,
	}, nil
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Unauthorized("invalid email or password")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get user: %v", err))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, apperror.Unauthorized("invalid email or password")
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, user.Email, user.DisplayName, Role(user.Role))
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User: UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Role:        Role(user.Role),
			Plan:        user.Plan,
			CreatedAt:   user.CreatedAt.Time,
		},
		Tokens: *tokens,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, req RefreshRequest) (*TokenPair, error) {
	tokenHash := hashToken(req.RefreshToken)

	stored, err := s.queries.GetRefreshToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.Unauthorized("invalid or expired refresh token")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get refresh token: %v", err))
	}

	// Delete old token (rotation)
	if err := s.queries.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("delete old refresh token: %v", err))
	}

	user, err := s.queries.GetUserByID(ctx, stored.UserID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get user for refresh: %v", err))
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, user.Email, user.DisplayName, Role(user.Role))
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (s *Service) Logout(ctx context.Context, req LogoutRequest) error {
	tokenHash := hashToken(req.RefreshToken)
	return s.queries.DeleteRefreshToken(ctx, tokenHash)
}

func toUserResponse(u sqlcgen.User) *UserResponse {
	return &UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        Role(u.Role),
		Plan:        u.Plan,
		CreatedAt:   u.CreatedAt.Time,
	}
}

func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*UserResponse, error) {
	u, err := s.queries.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("user", "not found")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get user: %v", err))
	}
	return &UserResponse{
		ID:          u.ID,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        Role(u.Role),
		Plan:        u.Plan,
		CreatedAt:   u.CreatedAt.Time,
	}, nil
}

// DeleteMe removes the user and all owned data. On-disk resources (git repos,
// collaborator clones, MinIO objects) are collected before the DB cascade and
// deleted best-effort afterwards.
func (s *Service) DeleteMe(ctx context.Context, userID uuid.UUID) error {
	// Collect all disk/object-store paths before the cascade wipes DB rows.
	repoPaths, err := s.queries.ListProjectGitPaths(ctx, userID)
	if err != nil {
		return apperror.Internal(fmt.Sprintf("list git repos: %v", err))
	}

	clonePaths, err := s.queries.ListUserCollaboratorClonePaths(ctx, userID)
	if err != nil {
		return apperror.Internal(fmt.Sprintf("list clone paths: %v", err))
	}

	var wikiKeys, exportKeys []string
	if s.storageClient != nil {
		rawWiki, e := s.queries.ListUserWikiImageKeys(ctx, userID)
		if e != nil {
			return apperror.Internal(fmt.Sprintf("list wiki image keys: %v", e))
		}
		wikiKeys = pgTextSlice(rawWiki)

		rawExport, e := s.queries.ListUserExportMinioKeys(ctx, userID)
		if e != nil {
			return apperror.Internal(fmt.Sprintf("list export minio keys: %v", e))
		}
		exportKeys = pgTextSlice(rawExport)
	}

	if err := s.queries.DeleteUser(ctx, userID); err != nil {
		return apperror.Internal(fmt.Sprintf("delete user: %v", err))
	}

	// Best-effort cleanup — errors are logged but don't fail the request.
	for _, path := range repoPaths {
		_ = os.RemoveAll(path)
	}
	for _, path := range clonePaths {
		_ = os.RemoveAll(path)
	}
	if s.storageClient != nil {
		for _, key := range wikiKeys {
			_ = s.storageClient.DeleteObject(ctx, key)
		}
		for _, key := range exportKeys {
			_ = s.storageClient.DeleteObject(ctx, key)
		}
	}

	return nil
}

func (s *Service) ValidateAccessToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, apperror.Unauthorized("invalid access token")
	}
	return claims, nil
}

func (s *Service) generateTokenPair(ctx context.Context, userID uuid.UUID, email, displayName string, role Role) (*TokenPair, error) {
	now := time.Now()

	claims := Claims{
		UserID:      userID,
		Email:       email,
		DisplayName: displayName,
		Role:        role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID.String(),
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("sign access token: %v", err))
	}

	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("generate refresh token: %v", err))
	}
	refreshToken := hex.EncodeToString(refreshBytes)

	_, err = s.queries.CreateRefreshToken(ctx, sqlcgen.CreateRefreshTokenParams{
		UserID:    userID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(s.refreshTokenExpiry), Valid: true},
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("store refresh token: %v", err))
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func pgTextSlice(ts []pgtype.Text) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		if t.Valid && t.String != "" {
			out = append(out, t.String)
		}
	}
	return out
}
