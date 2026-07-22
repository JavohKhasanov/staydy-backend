package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	pointsusecase "github.com/student-success/backend/internal/usecase/points"
	shopusecase "github.com/student-success/backend/internal/usecase/shop"
)

type studentShopItemResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Price int    `json:"price"`
	Owned bool   `json:"owned"`
}
type studentShopOutput struct{ Body []studentShopItemResponse }

// registerStudentShop mounts the student-facing shop + leaderboard. Mount on the student group.
func registerStudentShop(api huma.API, shop *shopusecase.Service, points *pointsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-shop-list",
		Method:      http.MethodGet,
		Path:        "/student/shop",
		Summary:     "Active shop items (with whether the student owns each)",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentShopOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		items, err := shop.ListForStudent(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student shop list failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentShopOutput{Body: make([]studentShopItemResponse, 0, len(items))}
		for _, it := range items {
			out.Body = append(out.Body, studentShopItemResponse{
				ID: it.ID.String(), Name: it.Name, Icon: it.Icon, Price: it.Price, Owned: it.Owned,
			})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "student-shop-buy",
		Method:        http.MethodPost,
		Path:          "/student/shop/{id}/buy",
		Summary:       "Buy a shop item with coins",
		Tags:          []string{"student"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusNotFound, http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := shop.Buy(ctx, p.OrgID, p.UserID, id); err != nil {
			return nil, mapShopError(err, log)
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-leaderboard",
		Method:      http.MethodGet,
		Path:        "/student/leaderboard",
		Summary:     "XP leaderboard (center-wide, or a group with ?groupId), with the student's own row flagged",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, in *leaderboardInput) (*leaderboardOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		gid, gerr := optionalGroupID(in.GroupID)
		if gerr != nil {
			return nil, gerr
		}
		rows, err := points.Leaderboard(ctx, p.OrgID, gid, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student leaderboard failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &leaderboardOutput{Body: toLeaderRows(rows)}, nil
	})
}
