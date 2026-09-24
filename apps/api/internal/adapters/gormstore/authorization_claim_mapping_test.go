package gormstore

import (
	"testing"
	"time"
)

func TestClaimedAuthorizationChangeRetainsAdmittedLease(t *testing.T) {
	row := authorizationOutboxModel{ID: "change", LockedBy: "path-api"}
	claimed := claimedAuthorizationChange(row, 2*time.Minute)
	if claimed.ID != row.ID || claimed.LockedBy != row.LockedBy || claimed.Lease != 2*time.Minute {
		t.Fatalf("claimed change = %+v", claimed)
	}
}
