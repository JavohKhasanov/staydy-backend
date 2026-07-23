package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	pointsusecase "github.com/student-success/backend/internal/usecase/points"
)

type gamificationBody struct {
	XPAttend      int `json:"xpAttend" minimum:"0" doc:"XP for on-time attendance"`
	XPLate        int `json:"xpLate" minimum:"0" doc:"XP for late attendance"`
	XPHomeworkMax int `json:"xpHomeworkMax" minimum:"0" doc:"max XP a teacher can give per homework"`
	XPCheckin     int `json:"xpCheckin" minimum:"0" doc:"XP for a weekly check-in"`
	LevelSize     int `json:"levelSize" minimum:"1" doc:"XP per level"`
	CoinBase      int `json:"coinBase" minimum:"0" doc:"coins per XP at level 1"`
	CoinStep      int `json:"coinStep" minimum:"0" doc:"extra coins per XP for each level above 1"`
}
type gamificationOutput struct{ Body gamificationBody }
type setGamificationInput struct{ Body gamificationBody }

type awardXPInput struct {
	ID   string `path:"id" format:"uuid"` // student id
	Body struct {
		XP     int    `json:"xp" doc:"XP to grant (may be negative to deduct)"`
		Kind   string `json:"kind,omitempty" enum:"exam,manual" doc:"exam pass or a manual bonus"`
		Reason string `json:"reason,omitempty" maxLength:"200"`
	}
}

// registerGamification mounts the center's gamification settings. Mount on director/manager.
func registerGamification(api huma.API, svc *pointsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "gamification-get",
		Method:      http.MethodGet,
		Path:        "/gamification",
		Summary:     "Get the center's gamification settings",
		Tags:        []string{"gamification"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*gamificationOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		cfg := svc.Config(ctx, p.OrgID)
		return &gamificationOutput{Body: toGamificationBody(cfg)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "gamification-set",
		Method:      http.MethodPut,
		Path:        "/gamification",
		Summary:     "Update the center's gamification settings",
		Tags:        []string{"gamification"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *setGamificationInput) (*gamificationOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		cfg := entity.GamificationConfig{
			XPAttend: in.Body.XPAttend, XPLate: in.Body.XPLate, XPHomeworkMax: in.Body.XPHomeworkMax,
			XPCheckin: in.Body.XPCheckin, LevelSize: in.Body.LevelSize,
			CoinBase: in.Body.CoinBase, CoinStep: in.Body.CoinStep,
		}
		if err := svc.SetConfig(ctx, p.OrgID, cfg); err != nil {
			if errors.Is(err, pointsusecase.ErrValidation) {
				return nil, huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
			}
			log.Error().Err(err).Msg("set gamification failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &gamificationOutput{Body: toGamificationBody(cfg)}, nil
	})
}

// registerManualXP mounts the teacher/admin one-off XP award (exam pass, manual bonus). Mount on the
// teaching group.
func registerManualXP(api huma.API, svc *pointsusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID:   "student-award-xp",
		Method:        http.MethodPost,
		Path:          "/students/{id}/xp",
		Summary:       "Award a student one-off XP (exam pass or manual bonus)",
		Tags:          []string{"gamification"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *awardXPInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		kind := entity.PointManual
		if in.Body.Kind == entity.PointExam {
			kind = entity.PointExam
		}
		// Unique ref so each manual award applies (not idempotent).
		ref := kind + ":" + in.Body.Reason + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if err := svc.AwardManual(ctx, p.OrgID, id, kind, in.Body.XP, ref); err != nil {
			log.Error().Err(err).Msg("manual xp award failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		return &noContentOutput{}, nil
	})
}

func toGamificationBody(c entity.GamificationConfig) gamificationBody {
	return gamificationBody{
		XPAttend: c.XPAttend, XPLate: c.XPLate, XPHomeworkMax: c.XPHomeworkMax, XPCheckin: c.XPCheckin,
		LevelSize: c.LevelSize, CoinBase: c.CoinBase, CoinStep: c.CoinStep,
	}
}
