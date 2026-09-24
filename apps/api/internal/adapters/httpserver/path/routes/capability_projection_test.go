package routes

import (
	"encoding/json"
	"net/http"
	"testing"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
)

func TestPathDetailAndListExposeServerAuthoritativeCapabilities(t *testing.T) {
	capabilities := pathapp.Capabilities{
		TrackTime:         true,
		InviteMembers:     true,
		ManageGoals:       true,
		ManageLifecycle:   false,
		TransferOwnership: true,
	}
	service := controlledService{capabilities: capabilities}

	detail := performGet(pathHandler(service), "private-path", "Bearer valid-application-session")
	var detailBody struct {
		Data struct {
			Capabilities pathapp.Capabilities `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &detailBody); err != nil {
		t.Fatal(err)
	}
	if detail.Code != http.StatusOK || detailBody.Data.Capabilities != capabilities {
		t.Fatalf("detail status=%d body=%s", detail.Code, detail.Body.String())
	}

	service.listProjections = []pathapp.Projection{{
		Path:         service.getEntity("private-path"),
		Capabilities: capabilities,
	}}
	list := performList(pathHandler(service), "Bearer valid-application-session")
	var listBody struct {
		Data []struct {
			Capabilities pathapp.Capabilities `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if list.Code != http.StatusOK || len(listBody.Data) != 1 || listBody.Data[0].Capabilities != capabilities {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
}
