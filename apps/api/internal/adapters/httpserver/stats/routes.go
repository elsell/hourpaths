package stats

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"net/http"
)

type Service interface {
	Get(context.Context, string, string, string, string) (domain.StatsSummary, error)
}
type statsInput struct {
	Authorization string `header:"Authorization"`
	Range         string `query:"range" default:"week" enum:"day,week,month,year,all_time"`
	Anchor        string `query:"anchor" maxLength:"10"`
	PathIDs       string `query:"pathIds" maxLength:"16384"`
}
type statsOutput struct {
	Body struct {
		Data domain.StatsSummary `json:"data"`
	}
}

func Register(api huma.API, service Service) {
	huma.Register(api, huma.Operation{OperationID: "get-stats", Method: http.MethodGet, Path: "/v1/stats", Summary: "Read personal recorded activity statistics", Tags: []string{"Stats"}, Security: []map[string][]string{{"oidc": {}}}}, func(ctx context.Context, input *statsInput) (*statsOutput, error) {
		result, err := service.Get(ctx, input.Authorization, input.Range, input.Anchor, input.PathIDs)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &statsOutput{}
		output.Body.Data = result
		return output, nil
	})
}
