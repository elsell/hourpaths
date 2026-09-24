package path

import (
	"errors"
	"testing"
	"time"
)

func TestNewInvitationPreservesExactlyTheOfferedOrdinaryRole(t *testing.T) {
	createdAt := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	for _, role := range []MembershipRole{RoleParticipant, RoleSupporter} {
		t.Run(string(role), func(t *testing.T) {
			invitation, err := NewInvitation(
				"invitation-1",
				"path-1",
				"inviter-1",
				"recipient-1",
				role,
				createdAt,
			)
			if err != nil {
				t.Fatalf("NewInvitation() error = %v", err)
			}
			if invitation.OfferedRole != role || !invitation.Pending() {
				t.Fatalf("NewInvitation() = %+v, want pending %q offer", invitation, role)
			}
			if !invitation.AcceptedAt.IsZero() {
				t.Fatalf("NewInvitation() accepted at = %v, want zero", invitation.AcceptedAt)
			}
		})
	}
}

func TestNewInvitationRejectsMalformedOrSelfDirectedOffers(t *testing.T) {
	now := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		id          InvitationID
		pathID      ID
		inviter     string
		recipient   string
		offeredRole MembershipRole
		createdAt   time.Time
	}{
		{name: "missing invitation id", pathID: "path-1", inviter: "inviter-1", recipient: "recipient-1", offeredRole: RoleParticipant, createdAt: now},
		{name: "missing path id", id: "invitation-1", inviter: "inviter-1", recipient: "recipient-1", offeredRole: RoleParticipant, createdAt: now},
		{name: "missing inviter", id: "invitation-1", pathID: "path-1", recipient: "recipient-1", offeredRole: RoleParticipant, createdAt: now},
		{name: "missing recipient", id: "invitation-1", pathID: "path-1", inviter: "inviter-1", offeredRole: RoleParticipant, createdAt: now},
		{name: "self invitation", id: "invitation-1", pathID: "path-1", inviter: "same-user", recipient: "same-user", offeredRole: RoleParticipant, createdAt: now},
		{name: "creator role", id: "invitation-1", pathID: "path-1", inviter: "inviter-1", recipient: "recipient-1", offeredRole: "creator", createdAt: now},
		{name: "administrator role", id: "invitation-1", pathID: "path-1", inviter: "inviter-1", recipient: "recipient-1", offeredRole: "administrator", createdAt: now},
		{name: "missing creation instant", id: "invitation-1", pathID: "path-1", inviter: "inviter-1", recipient: "recipient-1", offeredRole: RoleSupporter},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewInvitation(
				test.id,
				test.pathID,
				test.inviter,
				test.recipient,
				test.offeredRole,
				test.createdAt,
			)
			if !errors.Is(err, ErrInvalidFields) {
				t.Fatalf("NewInvitation() error = %v, want %v", err, ErrInvalidFields)
			}
		})
	}
}

func TestPathMemberRolesIncludeAdministratorWithoutBroadeningInvitations(t *testing.T) {
	if RoleAdministrator.Valid() {
		t.Fatal("administrator must not become an invitational role")
	}
	for _, role := range []MembershipRole{RoleParticipant, RoleSupporter, RoleAdministrator} {
		if !role.ValidPathMemberRole() {
			t.Fatalf("%q is not a valid Path member role", role)
		}
	}
	if MembershipRole("creator").ValidPathMemberRole() {
		t.Fatal("creator is a projection role, not a mutable membership role")
	}
}

func TestInvitationAcceptsOnlyTheIntendedRecipientAndNeverExpires(t *testing.T) {
	createdAt := time.Date(2026, 7, 23, 21, 0, 0, 0, time.UTC)
	invitation, err := NewInvitation(
		"invitation-1",
		"path-1",
		"inviter-1",
		"recipient-1",
		RoleParticipant,
		createdAt,
	)
	if err != nil {
		t.Fatalf("NewInvitation() error = %v", err)
	}

	if _, err := invitation.Accept("other-user", createdAt.Add(time.Minute)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("Accept(other user) error = %v, want opaque unavailable", err)
	}
	acceptedAt := createdAt.AddDate(10, 0, 0)
	accepted, err := invitation.Accept("recipient-1", acceptedAt)
	if err != nil {
		t.Fatalf("Accept(intended recipient after ten years) error = %v", err)
	}
	if accepted.Pending() || accepted.AcceptedAt != acceptedAt || accepted.OfferedRole != RoleParticipant {
		t.Fatalf("Accept() = %+v, want consumed participant invitation", accepted)
	}
	if invitation.AcceptedAt.IsZero() == false {
		t.Fatalf("Accept() mutated original invitation = %+v", invitation)
	}
	if _, err := accepted.Accept("recipient-1", acceptedAt.Add(time.Second)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("Accept(consumed) error = %v, want opaque unavailable", err)
	}
}

func TestInvitationRejectsOnlyTheIntendedRecipientOnce(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	rejectedAt := createdAt.Add(time.Minute)
	invitation, err := NewInvitation(
		"invitation-1", "path-1", "inviter-1", "recipient-1", RoleSupporter, createdAt,
	)
	if err != nil {
		t.Fatalf("NewInvitation() error = %v", err)
	}

	for _, test := range []struct {
		name      string
		recipient string
		at        time.Time
	}{
		{name: "wrong recipient", recipient: "recipient-2", at: rejectedAt},
		{name: "zero time", recipient: "recipient-1"},
		{name: "before creation", recipient: "recipient-1", at: createdAt.Add(-time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := invitation.Reject(test.recipient, test.at); !errors.Is(err, ErrInvitationUnavailable) {
				t.Fatalf("Reject() error = %v, want ErrInvitationUnavailable", err)
			}
		})
	}

	rejected, err := invitation.Reject("recipient-1", rejectedAt)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Pending() || rejected.RejectedAt != rejectedAt || !rejected.AcceptedAt.IsZero() {
		t.Fatalf("Reject() = %+v, want rejected terminal invitation", rejected)
	}
	if _, err := rejected.Reject("recipient-1", rejectedAt.Add(time.Second)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("second Reject() error = %v, want ErrInvitationUnavailable", err)
	}
	if _, err := rejected.Accept("recipient-1", rejectedAt.Add(time.Second)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("Accept() after rejection error = %v, want ErrInvitationUnavailable", err)
	}
}

func TestInvitationCancellationIsTerminal(t *testing.T) {
	created := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	invitation, err := NewInvitation("invite-1", "path-1", "sender", "recipient", RoleParticipant, created)
	if err != nil {
		t.Fatal(err)
	}
	canceled, err := invitation.Cancel(created.Add(time.Minute))
	if err != nil || canceled.CanceledAt.IsZero() || canceled.Pending() {
		t.Fatalf("Cancel() = %+v, %v", canceled, err)
	}
	if _, err := canceled.Accept("recipient", created.Add(2*time.Minute)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("Accept(canceled) error = %v", err)
	}
	if _, err := invitation.Cancel(created.Add(-time.Second)); !errors.Is(err, ErrInvitationUnavailable) {
		t.Fatalf("Cancel(before creation) error = %v", err)
	}
}
