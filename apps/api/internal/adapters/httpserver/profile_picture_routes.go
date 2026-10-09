package httpserver

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/profilepicture"
	"net/http"
)

type ownPictureDTO struct {
	UserID   string `json:"userId"`
	URL      string `json:"url"`
	Revision int64  `json:"revision"`
}
type pictureOutput struct {
	Body struct {
		Data ownPictureDTO `json:"data"`
	}
}

func ownPictureOutput(value app.OwnPicture) *pictureOutput {
	out := &pictureOutput{}
	out.Body.Data.UserID = value.Owner
	out.Body.Data.URL = value.URL
	out.Body.Data.Revision = value.Revision
	return out
}

type picturePreviewInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Image []byte `json:"image" maxLength:"13333336"`
	}
}
type picturePreviewDTO struct {
	Image  []byte `json:"image"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
type picturePreviewOutput struct {
	Body struct {
		Data picturePreviewDTO `json:"data"`
	}
}
type pictureCropDTO struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Size int `json:"size"`
}

func pictureCrop(value *pictureCropDTO) *profilepicture.Crop {
	if value == nil {
		return nil
	}
	return &profilepicture.Crop{X: value.X, Y: value.Y, Size: value.Size}
}

type pictureUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" minLength:"16" maxLength:"128"`
	Body           struct {
		Image            []byte          `json:"image" maxLength:"13333336"`
		Crop             *pictureCropDTO `json:"crop,omitempty"`
		Remove           bool            `json:"remove"`
		ExpectedRevision int64           `json:"expectedRevision" minimum:"1"`
	}
}
type publicPictureInput struct {
	ID string `path:"id"`
}
type publicPictureOutput struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	NoSniff      string `header:"X-Content-Type-Options"`
	Body         []byte
}

func registerPictureRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	admission := huma.Middlewares{func(ctx huma.Context, next func(huma.Context)) {
		admitted, err := application.AdmitProfilePictureUpload(ctx.Context(), ctx.Header("Authorization"))
		if err != nil {
			failure := mapError(err, false)
			status := http.StatusInternalServerError
			if classified, ok := failure.(huma.StatusError); ok {
				status = classified.GetStatus()
			}
			ctx.SetHeader("Content-Type", "application/problem+json")
			ctx.SetStatus(status)
			_ = json.NewEncoder(ctx.BodyWriter()).Encode(failure)
			return
		}
		next(huma.WithContext(ctx, admitted))
	}}
	huma.Register(api, huma.Operation{OperationID: "get-own-profile-picture", Method: http.MethodGet, Path: "/v1/me/profile/picture", Summary: "Read the signed-in account's profile picture", Security: security}, func(ctx context.Context, in *MeInput) (*pictureOutput, error) {
		value, err := application.OwnProfilePicture(ctx, in.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		return ownPictureOutput(value), nil
	})
	huma.Register(api, huma.Operation{OperationID: "preview-own-profile-picture", Method: http.MethodPost, Path: "/v1/me/profile/picture/preview", Summary: "Validate and preview a profile picture without saving", MaxBodyBytes: 13335000, Security: security, Middlewares: admission}, func(ctx context.Context, in *picturePreviewInput) (*picturePreviewOutput, error) {
		value, err := application.PreviewProfilePicture(ctx, in.Authorization, in.Body.Image)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &picturePreviewOutput{}
		out.Body.Data.Image = value.JPEG
		out.Body.Data.Width = value.Width
		out.Body.Data.Height = value.Height
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-own-profile-picture", Method: http.MethodPut, Path: "/v1/me/profile/picture", Summary: "Save or remove the signed-in account's profile picture", MaxBodyBytes: 13335000, Security: security, Middlewares: admission}, func(ctx context.Context, in *pictureUpdateInput) (*pictureOutput, error) {
		value, err := application.UpdateProfilePicture(ctx, in.Authorization, in.IdempotencyKey, app.PictureUpdate{Image: in.Body.Image, Crop: pictureCrop(in.Body.Crop), Remove: in.Body.Remove, ExpectedRevision: in.Body.ExpectedRevision})
		if err != nil {
			return nil, mapError(err, false)
		}
		return ownPictureOutput(value), nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-public-profile-picture", Method: http.MethodGet, Path: "/v1/profile-pictures/{id}", Summary: "Read a current application-hosted profile picture", Security: []map[string][]string{}, Responses: map[string]*huma.Response{"200": {Description: "Normalized JPEG picture", Content: map[string]*huma.MediaType{"image/jpeg": {Schema: &huma.Schema{Type: "string", Format: "binary"}}}}}}, func(ctx context.Context, in *publicPictureInput) (*publicPictureOutput, error) {
		data, err := application.PublicProfilePicture(ctx, in.ID)
		if err != nil {
			return nil, mapError(err, false)
		}
		return &publicPictureOutput{ContentType: "image/jpeg", CacheControl: "no-store", NoSniff: "nosniff", Body: data}, nil
	})
}
