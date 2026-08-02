package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrRefreshNotFound    = errors.New("refresh token not found")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Status       string
	AuthVersion  int
	CreatedAt    time.Time
}

type AuthState struct {
	ID          string
	Status      string
	AuthVersion int
}

type RefreshRecord struct {
	ID           string
	UserID       string
	JTI          string
	FamilyID     string
	TokenHash    []byte
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	ReplacedByID *string
}

type NewRefreshRecord struct {
	ID, UserID, JTI, FamilyID string
	TokenHash                 []byte
	ExpiresAt                 time.Time
	CreatedIP                 *string
	UserAgent                 *string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) InsertUser(ctx context.Context, tx pgx.Tx, user User) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, status, auth_version, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', 1, $4, $4)`,
		user.ID, user.Email, user.PasswordHash, user.CreatedAt.UTC())
	if err == nil {
		return nil
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "users_email_uidx" {
		return ErrEmailAlreadyExists
	}
	return fmt.Errorf("insert user: %w", err)
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, status, auth_version, created_at
		FROM users WHERE email = $1`, email))
}

func (r *Repository) FindAuthStateByID(ctx context.Context, userID string) (AuthState, error) {
	var state AuthState
	err := r.pool.QueryRow(ctx, `
		SELECT id, status, auth_version FROM users WHERE id = $1`, userID).
		Scan(&state.ID, &state.Status, &state.AuthVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return AuthState{}, ErrUserNotFound
	}
	if err != nil {
		return AuthState{}, fmt.Errorf("get user auth state: %w", err)
	}
	return state, nil
}

func (r *Repository) LockUserByID(ctx context.Context, tx pgx.Tx, userID string) (User, error) {
	return scanUser(tx.QueryRow(ctx, `
		SELECT id, email, password_hash, status, auth_version, created_at
		FROM users WHERE id = $1 FOR UPDATE`, userID))
}

type rowScanner interface {
	Scan(...any) error
}

func scanUser(row rowScanner) (User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Status, &user.AuthVersion, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}

func (r *Repository) InsertRefresh(ctx context.Context, tx pgx.Tx, record NewRefreshRecord, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (
			id, user_id, jti, family_id, token_hash, expires_at,
			created_ip, user_agent, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7::inet, $8, $9, $9)`,
		record.ID, record.UserID, record.JTI, record.FamilyID, record.TokenHash,
		record.ExpiresAt.UTC(), record.CreatedIP, record.UserAgent, now.UTC())
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (r *Repository) LockRefreshByHash(ctx context.Context, tx pgx.Tx, hash []byte) (RefreshRecord, error) {
	var record RefreshRecord
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, jti, family_id, token_hash, expires_at, revoked_at, replaced_by_token_id
		FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, hash).Scan(
		&record.ID, &record.UserID, &record.JTI, &record.FamilyID, &record.TokenHash,
		&record.ExpiresAt, &record.RevokedAt, &record.ReplacedByID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshRecord{}, ErrRefreshNotFound
	}
	if err != nil {
		return RefreshRecord{}, fmt.Errorf("lock refresh token: %w", err)
	}
	return record, nil
}

func (r *Repository) MarkRotated(ctx context.Context, tx pgx.Tx, oldID, replacementID string, now time.Time) error {
	tag, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET last_used_at = $2, revoked_at = $2, replaced_by_token_id = $3,
		    revocation_reason = 'rotated', updated_at = $2
		WHERE id = $1 AND revoked_at IS NULL`, oldID, now.UTC(), replacementID)
	if err != nil {
		return fmt.Errorf("mark refresh token rotated: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrRefreshNotFound
	}
	return nil
}

func (r *Repository) RevokeFamily(ctx context.Context, tx pgx.Tx, userID, familyID, reason string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = $4, revocation_reason = $3, updated_at = $4
		WHERE user_id = $1 AND family_id = $2 AND revoked_at IS NULL`,
		userID, familyID, reason, now.UTC())
	if err != nil {
		return fmt.Errorf("revoke refresh family: %w", err)
	}
	return nil
}

func (r *Repository) RevokeAllAndIncrementAuthVersion(ctx context.Context, tx pgx.Tx, userID, reason string, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = $3, revocation_reason = $2, updated_at = $3
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, reason, now.UTC()); err != nil {
		return fmt.Errorf("revoke user refresh tokens: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE users SET auth_version = auth_version + 1, updated_at = $2
		WHERE id = $1`, userID, now.UTC())
	if err != nil {
		return fmt.Errorf("increment auth version: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrUserNotFound
	}
	return nil
}

func (r *Repository) InsertSecurityAudit(ctx context.Context, tx pgx.Tx, auditID, userID, eventType string, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_events (id, actor_user_id, event_type, resource_type, resource_id, metadata, occurred_at)
		VALUES ($1, $2, $3, 'user', $2, '{}'::jsonb, $4)`,
		auditID, userID, eventType, now.UTC())
	if err != nil {
		return fmt.Errorf("insert security audit event: %w", err)
	}
	return nil
}
