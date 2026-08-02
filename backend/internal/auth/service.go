package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/database"
	"github.com/zxCroshka/GymTrackerAI/backend/internal/platform/id"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidRefresh     = errors.New("invalid refresh token")
)

type ProfileCreator interface {
	CreateDefaultProfile(context.Context, pgx.Tx, string, time.Time) error
}

type RegisterInput struct {
	Email    string
	Password string
}

type LoginInput = RegisterInput

type RequestMetadata struct {
	IP        *string
	UserAgent *string
}

type AuthResult struct {
	UserID          string    `json:"user_id"`
	Email           string    `json:"email"`
	AccessToken     string    `json:"access_token"`
	AccessExpiresAt time.Time `json:"access_expires_at"`
	RefreshToken    string    `json:"-"`
	RefreshExpires  time.Time `json:"-"`
}

type Service struct {
	pool       *pgxpool.Pool
	repository *Repository
	profiles   ProfileCreator
	hasher     PasswordHasher
	tokens     *TokenManager
	now        func() time.Time
	newID      func() (string, error)
	dummyHash  string
}

func NewService(pool *pgxpool.Pool, repository *Repository, profiles ProfileCreator, tokens *TokenManager) (*Service, error) {
	hasher := PasswordHasher{}
	dummyHash, err := hasher.Hash("constant-time-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("prepare password verifier: %w", err)
	}
	return &Service{
		pool: pool, repository: repository, profiles: profiles, hasher: hasher,
		tokens: tokens, now: time.Now, newID: id.UUID, dummyHash: dummyHash,
	}, nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput, metadata RequestMetadata) (AuthResult, error) {
	email, ok := normalizeEmail(input.Email)
	if !ok || !validPassword(input.Password) {
		return AuthResult{}, ErrInvalidInput
	}
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}
	userID, err := s.newID()
	if err != nil {
		return AuthResult{}, err
	}
	now := s.now().UTC()
	user := User{ID: userID, Email: email, PasswordHash: passwordHash, Status: "active", AuthVersion: 1, CreatedAt: now}
	var result AuthResult
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := s.repository.InsertUser(ctx, tx, user); err != nil {
			return err
		}
		if err := s.profiles.CreateDefaultProfile(ctx, tx, userID, now); err != nil {
			return err
		}
		issued, err := s.issuePair(ctx, tx, user, "", metadata, now)
		if err != nil {
			return err
		}
		result = issued
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	return result, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput, metadata RequestMetadata) (AuthResult, error) {
	email, ok := normalizeEmail(input.Email)
	if !ok || len(input.Password) > maximumPasswordBytes {
		s.hasher.Compare(s.dummyHash, input.Password)
		return AuthResult{}, ErrInvalidCredentials
	}
	user, err := s.repository.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			s.hasher.Compare(s.dummyHash, input.Password)
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, err
	}
	if user.Status != "active" || !s.hasher.Compare(user.PasswordHash, input.Password) {
		return AuthResult{}, ErrInvalidCredentials
	}
	now := s.now().UTC()
	var result AuthResult
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		issued, err := s.issuePair(ctx, tx, user, "", metadata, now)
		if err != nil {
			return err
		}
		result = issued
		return nil
	})
	if err != nil {
		return AuthResult{}, err
	}
	return result, nil
}

func (s *Service) issuePair(ctx context.Context, tx pgx.Tx, user User, familyID string, metadata RequestMetadata, now time.Time) (AuthResult, error) {
	var err error
	if familyID == "" {
		familyID, err = s.newID()
		if err != nil {
			return AuthResult{}, err
		}
	}
	accessJTI, err := s.newID()
	if err != nil {
		return AuthResult{}, err
	}
	refreshJTI, err := s.newID()
	if err != nil {
		return AuthResult{}, err
	}
	recordID, err := s.newID()
	if err != nil {
		return AuthResult{}, err
	}
	access, accessExpiry, err := s.tokens.issueAccess(user.ID, accessJTI, familyID, user.AuthVersion)
	if err != nil {
		return AuthResult{}, err
	}
	refresh, refreshExpiry, err := s.tokens.issueRefresh(user.ID, refreshJTI, familyID, user.AuthVersion)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.repository.InsertRefresh(ctx, tx, NewRefreshRecord{
		ID: recordID, UserID: user.ID, JTI: refreshJTI, FamilyID: familyID,
		TokenHash: tokenDigest(refresh), ExpiresAt: refreshExpiry,
		CreatedIP: metadata.IP, UserAgent: metadata.UserAgent,
	}, now); err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		UserID: user.ID, Email: user.Email, AccessToken: access, AccessExpiresAt: accessExpiry,
		RefreshToken: refresh, RefreshExpires: refreshExpiry,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, raw string, metadata RequestMetadata) (AuthResult, error) {
	claims, err := s.tokens.parseRefresh(raw)
	if err != nil {
		return AuthResult{}, ErrInvalidRefresh
	}
	now := s.now().UTC()
	var result AuthResult
	reuseDetected := false
	err = database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		record, err := s.repository.LockRefreshByHash(ctx, tx, tokenDigest(raw))
		if err != nil {
			if errors.Is(err, ErrRefreshNotFound) {
				return ErrInvalidRefresh
			}
			return err
		}
		if record.UserID != claims.Subject || record.JTI != claims.ID || record.FamilyID != claims.FamilyID {
			return ErrInvalidRefresh
		}
		user, err := s.repository.LockUserByID(ctx, tx, record.UserID)
		if err != nil {
			return err
		}
		if record.ReplacedByID != nil {
			if err := s.repository.RevokeAllAndIncrementAuthVersion(ctx, tx, user.ID, "refresh_reuse", now); err != nil {
				return err
			}
			auditID, err := s.newID()
			if err != nil {
				return err
			}
			if err := s.repository.InsertSecurityAudit(ctx, tx, auditID, user.ID, "auth.refresh_reuse", now); err != nil {
				return err
			}
			reuseDetected = true
			return nil
		}
		if record.RevokedAt != nil || !record.ExpiresAt.After(now) || user.Status != "active" || user.AuthVersion != claims.AuthVersion {
			return ErrInvalidRefresh
		}
		issued, err := s.issuePair(ctx, tx, user, record.FamilyID, metadata, now)
		if err != nil {
			return err
		}
		replacement, err := s.repository.LockRefreshByHash(ctx, tx, tokenDigest(issued.RefreshToken))
		if err != nil {
			return err
		}
		if err := s.repository.MarkRotated(ctx, tx, record.ID, replacement.ID, now); err != nil {
			return err
		}
		result = issued
		return nil
	})
	if err != nil || reuseDetected {
		if errors.Is(err, ErrInvalidRefresh) || reuseDetected {
			return AuthResult{}, ErrInvalidRefresh
		}
		return AuthResult{}, err
	}
	return result, nil
}

func (s *Service) Logout(ctx context.Context, actorID, raw string) error {
	claims, err := s.tokens.parseRefresh(raw)
	if err != nil || claims.Subject != actorID {
		return nil
	}
	now := s.now().UTC()
	return database.WithinTransaction(ctx, s.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		record, err := s.repository.LockRefreshByHash(ctx, tx, tokenDigest(raw))
		if errors.Is(err, ErrRefreshNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if record.UserID != actorID || record.FamilyID != claims.FamilyID {
			return nil
		}
		return s.repository.RevokeFamily(ctx, tx, actorID, record.FamilyID, "logout", now)
	})
}
