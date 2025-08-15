package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

type PostgresAccountRepo struct {
	db *pgxpool.Pool
}

func NewPostgresAccountRepo(db *pgxpool.Pool) *PostgresAccountRepo {
	return &PostgresAccountRepo{db: db}
}

func (r *PostgresAccountRepo) GetByUsername(ctx context.Context, username string) (*models.Account, error) {
	query := `
		SELECT id, username, password, access_level, created_at, updated_at 
		FROM accounts 
		WHERE username = $1`

	account := &models.Account{}
	err := r.db.QueryRow(ctx, query, username).Scan(
		&account.ID,
		&account.Username,
		&account.Password,
		&account.AccessLevel,
		&account.CreatedAt,
		&account.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return account, nil
}

func (r *PostgresAccountRepo) Create(ctx context.Context, account *models.Account) error {
	query := `
		INSERT INTO accounts (username, password, access_level, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	now := time.Now()
	account.CreatedAt = now
	account.UpdatedAt = now

	err := r.db.QueryRow(ctx, query,
		account.Username,
		account.Password,
		account.AccessLevel,
		account.CreatedAt,
		account.UpdatedAt,
	).Scan(&account.ID)

	return err
}

func (r *PostgresAccountRepo) Update(ctx context.Context, account *models.Account) error {
	query := `
		UPDATE accounts 
		SET username = $2, password = $3, access_level = $4, updated_at = $5
		WHERE id = $1`

	account.UpdatedAt = time.Now()

	_, err := r.db.Exec(ctx, query,
		account.ID,
		account.Username,
		account.Password,
		account.AccessLevel,
		account.UpdatedAt,
	)

	return err
}

func (r *PostgresAccountRepo) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM accounts WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		Account: NewPostgresAccountRepo(db),
	}
}
