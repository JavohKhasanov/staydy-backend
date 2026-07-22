package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	pointsusecase "github.com/student-success/backend/internal/usecase/points"
	shopusecase "github.com/student-success/backend/internal/usecase/shop"
)

type shopItemResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon,omitempty"`
	Price     int    `json:"price"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt"`
}
type shopItemBody struct {
	Name     string `json:"name" minLength:"1" maxLength:"100"`
	Icon     string `json:"icon,omitempty" maxLength:"16" doc:"emoji"`
	Price    int    `json:"price" minimum:"0"`
	IsActive bool   `json:"isActive,omitempty"`
}
type createShopItemInput struct{ Body shopItemBody }
type updateShopItemInput struct {
	ID   string `path:"id" format:"uuid"`
	Body shopItemBody
}

type leaderRowResponse struct {
	Rank      int    `json:"rank"`
	StudentID string `json:"studentId"`
	Name      string `json:"name"`
	XP        int    `json:"xp"`
	Coins     int    `json:"coins"`
	IsMe      bool   `json:"isMe,omitempty"`
}

type shopItemOutput struct{ Body shopItemResponse }
type shopItemsListOutput struct{ Body []shopItemResponse }
type leaderboardOutput struct{ Body []leaderRowResponse }

type leaderboardInput struct {
	GroupID string `query:"groupId" doc:"optional: rank within a group instead of the whole center"`
}

// registerShop mounts admin reward-shop management. Mount on a group gated to director/manager.
func registerShop(api huma.API, svc *shopusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "shop-items-list",
		Method:      http.MethodGet,
		Path:        "/shop/items",
		Summary:     "List reward-shop items",
		Tags:        []string{"shop"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*shopItemsListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		items, err := svc.ListItems(ctx, p.OrgID)
		if err != nil {
			return nil, mapShopError(err, log)
		}
		out := &shopItemsListOutput{Body: make([]shopItemResponse, 0, len(items))}
		for _, it := range items {
			out.Body = append(out.Body, toShopItemResponse(it))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "shop-item-create",
		Method:        http.MethodPost,
		Path:          "/shop/items",
		Summary:       "Create a reward-shop item",
		Tags:          []string{"shop"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createShopItemInput) (*shopItemOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		it, err := svc.CreateItem(ctx, p.OrgID, in.Body.Name, in.Body.Icon, in.Body.Price, in.Body.IsActive)
		if err != nil {
			return nil, mapShopError(err, log)
		}
		return &shopItemOutput{Body: toShopItemResponse(it)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "shop-item-update",
		Method:      http.MethodPut,
		Path:        "/shop/items/{id}",
		Summary:     "Update a reward-shop item",
		Tags:        []string{"shop"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateShopItemInput) (*shopItemOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		it, err := svc.UpdateItem(ctx, p.OrgID, id, in.Body.Name, in.Body.Icon, in.Body.Price, in.Body.IsActive)
		if err != nil {
			return nil, mapShopError(err, log)
		}
		return &shopItemOutput{Body: toShopItemResponse(it)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "shop-item-delete",
		Method:        http.MethodDelete,
		Path:          "/shop/items/{id}",
		Summary:       "Delete a reward-shop item",
		Tags:          []string{"shop"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteItem(ctx, p.OrgID, id); err != nil {
			return nil, mapShopError(err, log)
		}
		return &noContentOutput{}, nil
	})
}

// registerLeaderboard mounts the staff-facing leaderboard (director/manager/teacher). Students get
// their own leaderboard endpoint on the student group.
func registerLeaderboard(api huma.API, svc *pointsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "leaderboard",
		Method:      http.MethodGet,
		Path:        "/leaderboard",
		Summary:     "Student XP leaderboard (center-wide, or a group with ?groupId)",
		Tags:        []string{"gamification"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *leaderboardInput) (*leaderboardOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		gid, gerr := optionalGroupID(in.GroupID)
		if gerr != nil {
			return nil, gerr
		}
		rows, err := svc.Leaderboard(ctx, p.OrgID, gid, uuid.Nil)
		if err != nil {
			log.Error().Err(err).Msg("leaderboard failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &leaderboardOutput{Body: toLeaderRows(rows)}, nil
	})
}

func optionalGroupID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity("invalid groupId")
	}
	return &id, nil
}

func toLeaderRows(rows []entity.LeaderRow) []leaderRowResponse {
	out := make([]leaderRowResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, leaderRowResponse{
			Rank: r.Rank, StudentID: r.StudentID.String(), Name: r.Name, XP: r.XP, Coins: r.Coins, IsMe: r.IsMe,
		})
	}
	return out
}

func toShopItemResponse(it entity.ShopItem) shopItemResponse {
	return shopItemResponse{
		ID: it.ID.String(), Name: it.Name, Icon: it.Icon, Price: it.Price,
		IsActive: it.IsActive, CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func mapShopError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, shopusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
	case errors.Is(err, shopusecase.ErrNotFound):
		return huma.Error404NotFound("topilmadi")
	case errors.Is(err, shopusecase.ErrAlreadyOwned):
		return huma.Error409Conflict("Bu allaqachon sotib olingan")
	case errors.Is(err, shopusecase.ErrInsufficientCoins):
		return huma.Error422UnprocessableEntity("Kumush yetarli emas")
	default:
		log.Error().Err(err).Msg("shop op failed")
		return huma.Error500InternalServerError("internal error")
	}
}
