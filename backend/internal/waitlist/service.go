package waitlist

import (
	"context"

	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Service handles waitlist signup persistence.
type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// SignupResponse is the public wire shape for a waitlist entry.
type SignupResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	WhatTheyWrite string `json:"what_they_write"`
	CreatedAt     string `json:"created_at"`
}

// List returns all waitlist signups ordered by most-recent first.
func (s *Service) List(ctx context.Context) ([]SignupResponse, error) {
	rows, err := s.queries.ListWaitlistSignups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SignupResponse, len(rows))
	for i, r := range rows {
		out[i] = SignupResponse{
			ID:            r.ID.String(),
			Email:         r.Email,
			WhatTheyWrite: r.WhatTheyWrite,
			CreatedAt:     r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return out, nil
}

// Create inserts or updates a waitlist signup. Duplicate emails are accepted
// and update the what_they_write field (idempotent).
func (s *Service) Create(ctx context.Context, email, whatTheyWrite string) (SignupResponse, error) {
	row, err := s.queries.CreateWaitlistSignup(ctx, sqlcgen.CreateWaitlistSignupParams{
		Email:         email,
		WhatTheyWrite: whatTheyWrite,
	})
	if err != nil {
		return SignupResponse{}, err
	}
	return SignupResponse{
		ID:            row.ID.String(),
		Email:         row.Email,
		WhatTheyWrite: row.WhatTheyWrite,
		CreatedAt:     row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
