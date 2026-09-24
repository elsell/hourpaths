package spicedb

import (
	"context"
	"flag"
	"testing"
	"time"
)

var (
	liveEndpoint              = flag.String("spicedb-endpoint", "", "SpiceDB integration test endpoint")
	liveToken                 = flag.String("spicedb-token", "", "SpiceDB integration test token")
	liveInsecure              = flag.Bool("spicedb-insecure", false, "use plaintext transport for the SpiceDB integration test")
	acceptancePathID          = flag.String("acceptance-path-id", "", "private Path ID to seed for live HTTP acceptance")
	acceptancePathCreator     = flag.String("acceptance-path-creator", "", "private Path creator user ID")
	acceptancePathAdmin       = flag.String("acceptance-path-administrator", "", "private Path administrator user ID")
	acceptancePathParticipant = flag.String("acceptance-path-participant", "", "private Path participant user ID")
	acceptancePathSupporter   = flag.String("acceptance-path-supporter", "", "private Path supporter user ID")
)

func TestRuntimeCredentialReadsRequiredSchemaButCannotRewriteIt(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := authorizer.Health(ctx); err != nil {
		t.Fatalf("required schema was not healthy: %v", err)
	}
	if err := authorizer.WriteSchema(ctx, "definition user {}"); err == nil {
		t.Fatal("runtime credential rewrote the authorization schema")
	}
}

func TestPrivatePathViewPermissionMatrix(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "private-path-view-matrix"
	roles := []string{"creator", "administrator", "participant", "supporter"}
	for _, role := range roles {
		userID := role + "-user"
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			_ = authorizer.DeleteRelationship(cleanupCtx, "path", pathID, role, "user", userID)
		})
		allowed, err := authorizer.Check(ctx, "path", pathID, "view", userID)
		if err != nil || !allowed {
			t.Fatalf("%s view permission = %t, %v", role, allowed, err)
		}
	}
	allowed, err := authorizer.Check(ctx, "path", pathID, "view", "stranger-user")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("stranger received private Path view permission")
	}
}

func TestPathTrackingPermissionExcludesSupporters(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-tracking-matrix"
	for _, role := range []string{"creator", "administrator", "participant", "supporter"} {
		userID := "tracking-" + role
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role, err)
		}
		t.Cleanup(func() { _ = authorizer.DeleteRelationship(context.Background(), "path", pathID, role, "user", userID) })
		allowed, err := authorizer.Check(ctx, "path", pathID, "track", userID)
		if err != nil {
			t.Fatal(err)
		}
		if allowed != (role != "supporter") {
			t.Fatalf("%s track permission = %t", role, allowed)
		}
	}
}

func TestPathGoalManagementPermissionMatrix(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-goal-management-matrix"
	for _, role := range []struct {
		name    string
		allowed bool
	}{
		{name: "creator", allowed: true},
		{name: "administrator", allowed: true},
		{name: "participant", allowed: false},
		{name: "supporter", allowed: false},
	} {
		userID := "goal-management-" + role.name
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role.name, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role.name, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role.name, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			_ = authorizer.DeleteRelationship(cleanupCtx, "path", pathID, role.name, "user", userID)
		})
		allowed, err := authorizer.Check(ctx, "path", pathID, "manage_goals", userID)
		if err != nil {
			t.Fatalf("%s manage_goals permission: %v", role.name, err)
		}
		if allowed != role.allowed {
			t.Fatalf("%s manage_goals permission = %t, want %t", role.name, allowed, role.allowed)
		}
	}
	allowed, err := authorizer.Check(ctx, "path", pathID, "manage_goals", "goal-management-outsider")
	if err != nil {
		t.Fatalf("outsider manage_goals permission: %v", err)
	}
	if allowed {
		t.Fatal("outsider received Path goal-management permission")
	}
}

func TestPathRenamePermissionMatrix(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-rename-matrix"
	for _, role := range []struct {
		name    string
		allowed bool
	}{
		{name: "creator", allowed: true},
		{name: "administrator", allowed: true},
		{name: "participant", allowed: false},
		{name: "supporter", allowed: false},
	} {
		userID := "rename-" + role.name
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role.name, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role.name, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role.name, err)
		}
		t.Cleanup(func() {
			_ = authorizer.DeleteRelationship(context.Background(), "path", pathID, role.name, "user", userID)
		})
		allowed, err := authorizer.Check(ctx, "path", pathID, "rename", userID)
		if err != nil || allowed != role.allowed {
			t.Fatalf("%s rename permission = %t, %v; want %t", role.name, allowed, err, role.allowed)
		}
	}
	allowed, err := authorizer.Check(ctx, "path", pathID, "rename", "rename-outsider")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("outsider received Path rename permission")
	}
}

func TestPathMembershipManagementPermissionMatrix(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-membership-management-matrix"
	for _, role := range []struct {
		name    string
		allowed bool
	}{
		{name: "creator", allowed: true},
		{name: "administrator", allowed: true},
		{name: "participant", allowed: false},
		{name: "supporter", allowed: false},
	} {
		userID := "membership-management-" + role.name
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role.name, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role.name, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role.name, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			_ = authorizer.DeleteRelationship(cleanupCtx, "path", pathID, role.name, "user", userID)
		})
		allowed, err := authorizer.Check(ctx, "path", pathID, "manage_members", userID)
		if err != nil {
			t.Fatalf("%s manage_members permission: %v", role.name, err)
		}
		if allowed != role.allowed {
			t.Fatalf("%s manage_members permission = %t, want %t", role.name, allowed, role.allowed)
		}
	}
	allowed, err := authorizer.Check(ctx, "path", pathID, "manage_members", "membership-management-outsider")
	if err != nil {
		t.Fatalf("outsider manage_members permission: %v", err)
	}
	if allowed {
		t.Fatal("outsider received Path membership-management permission")
	}
}

func TestPathAdministratorLifecyclePermissionMatrix(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-administrator-lifecycle-matrix"
	for _, role := range []struct {
		name                   string
		canManage, canStepDown bool
	}{
		{name: "creator", canManage: true},
		{name: "administrator", canStepDown: true},
		{name: "participant"},
		{name: "supporter"},
	} {
		userID := "administrator-lifecycle-" + role.name
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role.name, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role.name, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role.name, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			_ = authorizer.DeleteRelationship(cleanupCtx, "path", pathID, role.name, "user", userID)
		})
		for permission, want := range map[string]bool{"manage_administrators": role.canManage, "step_down_administrator": role.canStepDown} {
			allowed, err := authorizer.Check(ctx, "path", pathID, permission, userID)
			if err != nil || allowed != want {
				t.Fatalf("%s %s permission=%t err=%v want=%t", role.name, permission, allowed, err, want)
			}
		}
	}
}

func TestPathLifecycleManagementPermissionIsCreatorOnly(t *testing.T) {
	if *liveEndpoint == "" {
		t.Skip("-spicedb-endpoint is required for live SpiceDB integration")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const pathID = "path-lifecycle-management-matrix"
	for _, role := range []struct {
		name    string
		allowed bool
	}{
		{name: "creator", allowed: true},
		{name: "administrator", allowed: false},
		{name: "participant", allowed: false},
		{name: "supporter", allowed: false},
	} {
		userID := "lifecycle-management-" + role.name
		_ = authorizer.DeleteRelationship(ctx, "path", pathID, role.name, "user", userID)
		if err := authorizer.WriteRelationship(ctx, "path", pathID, role.name, "user", userID); err != nil {
			t.Fatalf("write %s relationship: %v", role.name, err)
		}
		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cleanupCancel()
			_ = authorizer.DeleteRelationship(cleanupCtx, "path", pathID, role.name, "user", userID)
		})
		allowed, err := authorizer.Check(ctx, "path", pathID, "manage_lifecycle", userID)
		if err != nil {
			t.Fatalf("%s manage_lifecycle permission: %v", role.name, err)
		}
		if allowed != role.allowed {
			t.Fatalf("%s manage_lifecycle permission = %t, want %t", role.name, allowed, role.allowed)
		}
	}
	allowed, err := authorizer.Check(ctx, "path", pathID, "manage_lifecycle", "lifecycle-management-outsider")
	if err != nil {
		t.Fatalf("outsider manage_lifecycle permission: %v", err)
	}
	if allowed {
		t.Fatal("outsider received Path lifecycle-management permission")
	}
}

func TestSeedPrivatePathHTTPAcceptanceRelationships(t *testing.T) {
	if *acceptancePathID == "" {
		t.Skip("-acceptance-path-id is required for the live HTTP acceptance fixture")
	}
	if *liveEndpoint == "" || *acceptancePathCreator == "" || *acceptancePathAdmin == "" || *acceptancePathParticipant == "" || *acceptancePathSupporter == "" {
		t.Fatal("complete SpiceDB endpoint and private Path role IDs are required")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	assignments := []struct{ role, userID string }{
		{"creator", *acceptancePathCreator},
		{"administrator", *acceptancePathAdmin},
		{"participant", *acceptancePathParticipant},
		{"supporter", *acceptancePathSupporter},
	}
	for _, assignment := range assignments {
		if err := authorizer.WriteRelationship(ctx, "path", *acceptancePathID, assignment.role, "user", assignment.userID); err != nil {
			t.Fatalf("seed %s relationship: %v", assignment.role, err)
		}
	}
}

func TestSeedPathHTTPAcceptanceParticipant(t *testing.T) {
	if *acceptancePathID == "" {
		t.Skip("-acceptance-path-id is required for the live HTTP acceptance fixture")
	}
	if *liveEndpoint == "" || *acceptancePathParticipant == "" {
		t.Fatal("SpiceDB endpoint and Path participant ID are required")
	}
	authorizer, err := New(*liveEndpoint, *liveToken, *liveInsecure)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := authorizer.WriteRelationship(ctx, "path", *acceptancePathID, "participant", "user", *acceptancePathParticipant); err != nil {
		t.Fatalf("seed participant relationship: %v", err)
	}
	allowed, err := authorizer.Check(ctx, "path", *acceptancePathID, "view", *acceptancePathParticipant)
	if err != nil || !allowed {
		t.Fatalf("seeded participant view permission = %t, %v", allowed, err)
	}
}
