// Package shop is the reward-shop business logic: a center manages items; students buy them with
// coins (atomic spend + purchase in the repo).
package shop

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("shop: validation failed")
	ErrNotFound   = errors.New("shop: not found")
	// ErrAlreadyOwned / ErrInsufficientCoins surface purchase failures to the caller.
	ErrAlreadyOwned      = errors.New("shop: already owned")
	ErrInsufficientCoins = errors.New("shop: not enough coins")
)

type Service struct {
	repo repo.ShopRepository
}

func NewService(r repo.ShopRepository) *Service { return &Service{repo: r} }

func (s *Service) CreateItem(ctx context.Context, orgID uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error) {
	name = strings.TrimSpace(name)
	if name == "" || price < 0 {
		return entity.ShopItem{}, ErrValidation
	}
	return s.repo.CreateItem(ctx, orgID, name, strings.TrimSpace(icon), price, active)
}

func (s *Service) ListItems(ctx context.Context, orgID uuid.UUID) ([]entity.ShopItem, error) {
	return s.repo.ListItems(ctx, orgID)
}

func (s *Service) UpdateItem(ctx context.Context, orgID, id uuid.UUID, name, icon string, price int, active bool) (entity.ShopItem, error) {
	name = strings.TrimSpace(name)
	if name == "" || price < 0 {
		return entity.ShopItem{}, ErrValidation
	}
	it, err := s.repo.UpdateItem(ctx, orgID, id, name, strings.TrimSpace(icon), price, active)
	if errors.Is(err, repo.ErrNotFound) {
		return entity.ShopItem{}, ErrNotFound
	}
	return it, err
}

func (s *Service) DeleteItem(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteItem(ctx, orgID, id)
}

func (s *Service) ListForStudent(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.StudentShopItem, error) {
	return s.repo.ListForStudent(ctx, orgID, studentID)
}

// Buy purchases an active item for a student, deducting coins atomically.
func (s *Service) Buy(ctx context.Context, orgID, studentID, itemID uuid.UUID) error {
	item, err := s.repo.GetItem(ctx, orgID, itemID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if !item.IsActive {
		return ErrNotFound
	}
	err = s.repo.BuyItem(ctx, orgID, studentID, itemID, item.Price)
	switch {
	case errors.Is(err, repo.ErrAlreadyExists):
		return ErrAlreadyOwned
	case errors.Is(err, repo.ErrInsufficientCoins):
		return ErrInsufficientCoins
	default:
		return err
	}
}
