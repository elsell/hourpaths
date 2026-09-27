package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"net/http"
)

type AppearanceService interface {
	ReadAppearance(context.Context, string, domain.ID) (domain.Appearance, error)
	SaveAppearance(context.Context, string, string, domain.ID, domain.Appearance) (domain.Appearance, error)
}
type PathAppearance struct {
	Color    string `json:"color,omitempty" enum:"coral,lavender,gold,mint,blue,pink"`
	Emoji    string `json:"emoji,omitempty" maxLength:"32"`
	Revision int64  `json:"revision" minimum:"0"`
}
type AppearanceReadInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathID" minLength:"1"`
}
type AppearanceSaveInput struct {
	Authorization  string `header:"Authorization"`
	PathID         string `path:"pathID" minLength:"1"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		Color            string `json:"color" required:"true" enum:"coral,lavender,gold,mint,blue,pink"`
		Emoji            string `json:"emoji" required:"true" minLength:"1" maxLength:"32"`
		ExpectedRevision int64  `json:"expectedRevision" required:"true" minimum:"0"`
	}
}
type AppearanceOutput struct {
	Body struct {
		Data PathAppearance `json:"data"`
	}
}

func appearanceOutput(value domain.Appearance) *AppearanceOutput {
	out := &AppearanceOutput{}
	out.Body.Data = PathAppearance{Color: value.Color, Emoji: value.Emoji, Revision: value.Revision}
	return out
}
func registerAppearance(api huma.API, service AppearanceService, security []map[string][]string) {
	huma.Register(api, huma.Operation{OperationID: "read-path-appearance", Method: http.MethodGet, Path: "/v1/me/path-appearances/{pathID}", Security: security}, func(ctx context.Context, input *AppearanceReadInput) (*AppearanceOutput, error) {
		value, err := service.ReadAppearance(ctx, input.Authorization, domain.ID(input.PathID))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return appearanceOutput(value), nil
	})
	huma.Register(api, huma.Operation{OperationID: "save-path-appearance", Method: http.MethodPut, Path: "/v1/me/path-appearances/{pathID}", Security: security}, func(ctx context.Context, input *AppearanceSaveInput) (*AppearanceOutput, error) {
		value, err := service.SaveAppearance(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), domain.Appearance{Color: input.Body.Color, Emoji: input.Body.Emoji, Revision: input.Body.ExpectedRevision})
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return appearanceOutput(value), nil
	})
}
