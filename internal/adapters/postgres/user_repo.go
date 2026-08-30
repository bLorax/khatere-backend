// Package postgres holds adapters. Each adapter implements one domain port
// with Postgres queries. The rest of the code never imports database/sql
// directly. The rest of the code only imports the domain ports.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	domainuser "yadegar/internal/domain/user"
)

// UserRepository implements domainuser.Repository with Postgres.
type UserRepository struct {
	conn *sql.DB
}

func NewUserRepository(conn *sql.DB) *UserRepository {
	return &UserRepository{conn: conn}
}

func (r *UserRepository) Create(ctx context.Context, u *domainuser.User) error {
	_, err := r.conn.ExecContext(ctx,
		`INSERT INTO users (id, username, email, password_hash) VALUES ($1, $2, $3, $4)`,
		u.ID, u.Username, u.Email, u.PasswordHash,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domainuser.ErrUsernameTaken
		}
		return err
	}
	return nil
}

func (r *UserRepository) FindByIdentifier(ctx context.Context, identifier string) (*domainuser.User, error) {
	var u domainuser.User
	err := r.conn.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = $1 OR email = $1`,
		identifier,
	).Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainuser.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
