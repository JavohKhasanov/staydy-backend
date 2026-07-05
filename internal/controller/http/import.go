package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

type importRowRequest struct {
	Name         string `json:"name" minLength:"1" maxLength:"255"`
	Date         string `json:"date" format:"date" doc:"YYYY-MM-DD"`
	Present      *bool  `json:"present,omitempty" doc:"omit to leave attendance untouched"`
	HomeworkDone *bool  `json:"homeworkDone,omitempty" doc:"omit to leave homework untouched"`
}

type importRequest struct {
	Rows []importRowRequest `json:"rows" minItems:"1" maxItems:"5000"`
}

type importInput struct{ Body importRequest }

type skippedRowResponse struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type importOutput struct {
	Body struct {
		Imported int                  `json:"imported"`
		Skipped  []skippedRowResponse `json:"skipped"`
	}
}

// registerImport mounts the bulk CRM-with import. Mount on a group gated to center_admin /
// super_admin. Clients (e.g. the admin app's CSV importer) parse the file and POST rows here.
func registerImport(api huma.API, students *studentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "import-records",
		Method:      http.MethodPost,
		Path:        "/import/records",
		Summary:     "Bulk-import attendance/homework (students matched by name)",
		Tags:        []string{"import"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *importInput) (*importOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		rows := make([]studentusecase.ImportRow, 0, len(in.Body.Rows))
		for _, r := range in.Body.Rows {
			date, derr := time.Parse("2006-01-02", r.Date)
			if derr != nil {
				return nil, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD: " + r.Date)
			}
			rows = append(rows, studentusecase.ImportRow{
				Name:         r.Name,
				Date:         date,
				Present:      r.Present,
				HomeworkDone: r.HomeworkDone,
			})
		}
		result, err := students.ImportRecords(ctx, p.OrgID, rows)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		out := &importOutput{}
		out.Body.Imported = result.Imported
		out.Body.Skipped = make([]skippedRowResponse, 0, len(result.Skipped))
		for _, sk := range result.Skipped {
			out.Body.Skipped = append(out.Body.Skipped, skippedRowResponse{Name: sk.Name, Reason: sk.Reason})
		}
		return out, nil
	})
}
