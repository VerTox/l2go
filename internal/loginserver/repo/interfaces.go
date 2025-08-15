package repo

import (
	"context"

	"github.com/VerTox/l2go/internal/loginserver/models"
)

type AccountRepository interface {
	GetByUsername(ctx context.Context, username string) (*models.Account, error)
	Create(ctx context.Context, account *models.Account) error
	Update(ctx context.Context, account *models.Account) error
	Delete(ctx context.Context, id int) error
}

type Repository struct {
	Account AccountRepository
}
