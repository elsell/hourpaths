package main

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	pathroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/routes"
	socialroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/routes"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/generated"
	"os"
)

func main() {
	registrations := generated.Registrations(generated.Dependencies{})
	registrations = append(registrations, func(api huma.API) {
		socialroutes.Register(api, socialapp.New(socialapp.Dependencies{}))
	})
	registrations = append(registrations, func(api huma.API) {
		pathroutes.RegisterOwnershipTransfers(api, pathapp.NewOwnershipTransferService(pathapp.OwnershipTransferDependencies{}))
	})
	_, api := httpserver.New(app.App{}, generated.Domains, httpserver.Options{OIDCIssuer: "https://id.example.invalid", OIDCAuthorizationURL: "https://id.example.invalid/authorize", OIDCTokenURL: "https://id.example.invalid/token", OIDCDocsClientID: "hourpaths-docs", OIDCDocsRedirectURI: "https://api.example.invalid/docs", PublicBaseURL: "https://api.example.invalid", DomainRegistrations: registrations})
	if err := json.NewEncoder(os.Stdout).Encode(api.OpenAPI()); err != nil {
		panic(err)
	}
}
