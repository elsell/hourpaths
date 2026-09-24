package spicedb

import (
	"strings"
	"testing"
)

func TestRequiredSchemaDefinesPrivatePathViewRoles(t *testing.T) {
	for _, required := range []string{
		"definition user",
		"relation follower: user",
		"definition path",
		"relation creator: user",
		"relation administrator: user",
		"relation participant: user",
		"relation supporter: user",
		"relation followers_owner: user",
		"relation public_viewer: user:*",
		"permission view = creator + administrator + participant + supporter + followers_owner->follower + public_viewer",
		"permission track = creator + administrator + participant",
		"permission manage_goals = creator + administrator",
		"permission rename = creator + administrator",
		"permission manage_members = creator + administrator",
		"permission manage_administrators = creator",
		"permission step_down_administrator = administrator",
		"permission manage_lifecycle = creator",
		"permission manage_visibility = creator",
		"permission transfer_ownership = creator",
		"permission leave = administrator + participant + supporter",
	} {
		if !strings.Contains(RequiredSchema, required) {
			t.Fatalf("Path authorization schema is missing %q", required)
		}
	}
}

func TestRequiredSchemaComparisonRejectsDrift(t *testing.T) {
	if !schemaCurrent(RequiredSchema) {
		t.Fatal("required schema rejected itself")
	}
	userDefinition := "definition user { relation follower: user }"
	pathDefinition := "definition path { relation creator: user relation administrator: user relation participant: user relation supporter: user relation followers_owner: user relation public_viewer: user:* permission view = creator + administrator + participant + supporter + followers_owner->follower + public_viewer permission track = creator + administrator + participant permission rename = creator + administrator permission manage_goals = creator + administrator permission manage_members = creator + administrator permission manage_administrators = creator permission step_down_administrator = administrator permission manage_lifecycle = creator permission manage_visibility = creator permission transfer_ownership = creator permission leave = administrator + participant + supporter }"
	resourceDefinition := "definition resource { relation owner: user permission view = owner permission update = owner permission delete = owner }"
	if !schemaCurrent(userDefinition + "\n\n" + resourceDefinition + "\n" + pathDefinition) {
		t.Fatal("equivalent provider formatting rejected")
	}
	if !schemaCurrent(pathDefinition + "\n" + resourceDefinition + "\n" + userDefinition) {
		t.Fatal("equivalent provider ordering rejected")
	}
	if schemaCurrent("definition user {}") {
		t.Fatal("incomplete schema accepted")
	}
	if schemaCurrent(RequiredSchema + "\ndefinition attacker {}") {
		t.Fatal("unexpected schema expansion accepted")
	}
}
