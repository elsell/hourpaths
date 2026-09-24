package pathstore

import (
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestNotificationProjectionSelectsPersistedVisibilitySnapshot(t *testing.T) {
	database, err := gorm.Open(postgres.Open("host=127.0.0.1 user=unused dbname=unused sslmode=disable"), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	if err != nil {
		t.Fatal(err)
	}
	statement := notificationProjectionQuery(database).
		Find(&[]invitationNotificationRow{}).Statement
	if sql := statement.SQL.String(); !strings.Contains(sql, "COALESCE(notification_models.path_visibility, '') AS path_visibility") {
		t.Fatalf("notification projection omitted persisted visibility snapshot: %s", sql)
	}
}
