package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
)

// AuthDB is the PostgreSQL surface used by AuthRepo.
type AuthDB interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

// AuthRepo persists staff accounts. Sessions live in Redis, not here.
type AuthRepo struct {
	db AuthDB
}

// NewAuthRepo creates an AuthRepo backed by db.
func NewAuthRepo(db AuthDB) *AuthRepo {
	return &AuthRepo{db: db}
}

// GetAdminUserByLogin returns a staff account by its unique login.
func (r *AuthRepo) GetAdminUserByLogin(ctx context.Context, login string) (*models.AdminUser, error) {
	const fc = "authRepo_GetAdminUserByLogin"

	query := `SELECT id, login, password_hash, role FROM admin_users WHERE login = $1`

	return r.getAdminUser(ctx, fc, query, login)
}

// GetAdminUserByID returns a staff account by its primary key.
func (r *AuthRepo) GetAdminUserByID(ctx context.Context, id uuid.UUID) (*models.AdminUser, error) {
	const fc = "authRepo_GetAdminUserByID"

	query := `SELECT id, login, password_hash, role FROM admin_users WHERE id = $1`

	return r.getAdminUser(ctx, fc, query, id)
}

// getAdminUser runs a single-account lookup and maps a missing row to ErrUnauthorized.
func (r *AuthRepo) getAdminUser(
	ctx context.Context,
	fc string,
	query string,
	argument any,
) (*models.AdminUser, error) {
	var user models.AdminUser
	if err := r.db.QueryRow(ctx, query, argument).Scan(
		&user.ID,
		&user.Login,
		&user.PasswordHash,
		&user.Role,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", fc, errs.ErrUnauthorized)
		}

		return nil, fmt.Errorf("%s: %w", fc, err)
	}

	return &user, nil
}
