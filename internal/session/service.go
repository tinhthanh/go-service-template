// Package session is who is signed in: signing in by email and password,
// the session a cookie names, a longer one from the refresh cookie, and
// signing out. A session is a random token; the store keeps only its hash,
// so a copy of the table is no session taken. It is the platform's
// auth.Authenticator, which internal/server passes to the middleware.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tinhthanh/go-service-template/internal/platform/auth"
	"github.com/tinhthanh/go-service-template/internal/platform/problem"
)

const _tokenBytes = 32

// Account is a person as signing in reads them.
type Account struct {
	ID           string
	PasswordHash string
}

// Pair is a session's two tokens, as hashes, and when each ends.
type Pair struct {
	TokenHash     []byte
	RefreshHash   []byte
	EndsAt        time.Time
	RefreshEndsAt time.Time
}

// Store is what the service keeps sessions in (store.go, on Postgres).
type Store interface {
	// Account is the person with this email, any case, or problem.ErrNotFound.
	Account(ctx context.Context, email string) (Account, error)
	Create(ctx context.Context, userID string, p Pair) error
	// Person is who a live session names, or problem.ErrNotFound.
	Person(ctx context.Context, tokenHash []byte) (auth.Principal, error)
	// Renew swaps a live refresh token's session for the new pair, or problem.ErrNotFound.
	Renew(ctx context.Context, refreshHash []byte, next Pair) error
	Delete(ctx context.Context, tokenHash []byte) error
}

// Lengths are how long a session and its refresh last.
type Lengths struct {
	Session time.Duration
	Refresh time.Duration
}

// Tokens are what the cookies carry, and when each ends.
type Tokens struct {
	Session       string
	Refresh       string
	EndsAt        time.Time
	RefreshEndsAt time.Time
}

// Service -.
type Service struct {
	store   Store
	lengths Lengths
	now     func() time.Time
	// dummy is a hash to compare against when no account has the email,
	// so a wrong email takes as long as a wrong password.
	dummy []byte
}

// New is the service; now is the clock (time.Now, or a test's).
func New(store Store, lengths Lengths, now func() time.Time) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("not anyone's password"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}

	return &Service{store: store, lengths: lengths, now: now, dummy: dummy}, nil
}

// SignIn starts a session for the email and password. A wrong email and a
// wrong password are refused alike, so the answer says nothing of who has
// an account.
func (s *Service) SignIn(ctx context.Context, email, password string) (Tokens, auth.Principal, error) {
	acc, err := s.store.Account(ctx, strings.TrimSpace(email))

	switch {
	case errors.Is(err, problem.ErrNotFound):
		_ = bcrypt.CompareHashAndPassword(s.dummy, []byte(password))

		return Tokens{}, auth.Principal{}, problem.ErrUnauthorized
	case err != nil:
		return Tokens{}, auth.Principal{}, fmt.Errorf("session: sign in: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(password)) != nil {
		return Tokens{}, auth.Principal{}, problem.ErrUnauthorized
	}

	tokens, pair, err := s.newPair()
	if err != nil {
		return Tokens{}, auth.Principal{}, err
	}

	if err := s.store.Create(ctx, acc.ID, pair); err != nil {
		return Tokens{}, auth.Principal{}, fmt.Errorf("session: sign in: %w", err)
	}

	p, err := s.Authenticate(ctx, tokens.Session)

	return tokens, p, err
}

// Authenticate is who a session token names, while the session lasts.
func (s *Service) Authenticate(ctx context.Context, token string) (auth.Principal, error) {
	p, err := s.store.Person(ctx, hash(token))
	if errors.Is(err, problem.ErrNotFound) {
		return auth.Principal{}, problem.ErrUnauthorized
	}

	if err != nil {
		return auth.Principal{}, fmt.Errorf("session: %w", err)
	}

	return p, nil
}

// Refresh renews the session a live refresh token names, with new tokens
// for both: the old ones stop working.
func (s *Service) Refresh(ctx context.Context, refresh string) (Tokens, auth.Principal, error) {
	if refresh == "" {
		return Tokens{}, auth.Principal{}, problem.ErrUnauthorized
	}

	tokens, pair, err := s.newPair()
	if err != nil {
		return Tokens{}, auth.Principal{}, err
	}

	err = s.store.Renew(ctx, hash(refresh), pair)
	if errors.Is(err, problem.ErrNotFound) {
		return Tokens{}, auth.Principal{}, problem.ErrUnauthorized
	}

	if err != nil {
		return Tokens{}, auth.Principal{}, fmt.Errorf("session: refresh: %w", err)
	}

	p, err := s.Authenticate(ctx, tokens.Session)

	return tokens, p, err
}

// SignOut ends the session the token names; none is no error.
func (s *Service) SignOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	if err := s.store.Delete(ctx, hash(token)); err != nil {
		return fmt.Errorf("session: sign out: %w", err)
	}

	return nil
}

func (s *Service) newPair() (Tokens, Pair, error) {
	session, err := random()
	if err != nil {
		return Tokens{}, Pair{}, err
	}

	refresh, err := random()
	if err != nil {
		return Tokens{}, Pair{}, err
	}

	now := s.now()
	t := Tokens{Session: session, Refresh: refresh, EndsAt: now.Add(s.lengths.Session), RefreshEndsAt: now.Add(s.lengths.Refresh)}

	return t, Pair{TokenHash: hash(session), RefreshHash: hash(refresh), EndsAt: t.EndsAt, RefreshEndsAt: t.RefreshEndsAt}, nil
}

func random() (string, error) {
	b := make([]byte, _tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("session: a token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hash(token string) []byte {
	sum := sha256.Sum256([]byte(token))

	return sum[:]
}
