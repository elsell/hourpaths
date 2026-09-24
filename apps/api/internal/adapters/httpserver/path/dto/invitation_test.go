package dto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPracticeReactionNotificationDTOUsesExactPublicContext(t *testing.T) {
	value := PathInvitationNotification{
		ID: "notice-1", Type: "practice_reaction", Presentation: "informational",
		Actor:  PathInvitationPublicIdentity{UserID: "reactor", Username: "reader", DisplayName: "Reader"},
		PathID: "path-1", PathName: "Piano", SocialFeedEventID: "practice:activity-1", Reaction: "fire",
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"pathId":"path-1"`, `"pathName":"Piano"`, `"socialFeedEventId":"practice:activity-1"`, `"reaction":"fire"`} {
		if !strings.Contains(string(encoded), expected) {
			t.Fatalf("missing %s in %s", expected, encoded)
		}
	}
	for _, forbidden := range []string{`"invitationId"`, `"ownershipTransferId"`, `"followRequestId"`, `"offeredRole"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("unexpected %s in %s", forbidden, encoded)
		}
	}

	typeField, _ := reflect.TypeOf(value).FieldByName("Type")
	reactionField, _ := reflect.TypeOf(value).FieldByName("Reaction")
	if !strings.Contains(typeField.Tag.Get("enum"), "practice_reaction") || reactionField.Tag.Get("enum") != "heart,applause,fire,strong,celebrate" {
		t.Fatalf("notification enums type=%q reaction=%q", typeField.Tag.Get("enum"), reactionField.Tag.Get("enum"))
	}
}
