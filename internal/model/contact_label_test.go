package model

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestParseContactLabelIDs(t *testing.T) {
	extraBuffer := protowire.AppendTag(nil, 4, protowire.BytesType)
	extraBuffer = protowire.AppendString(extraBuffer, "ignored")
	extraBuffer = protowire.AppendTag(extraBuffer, contactLabelFieldNumber, protowire.BytesType)
	extraBuffer = protowire.AppendString(extraBuffer, "72,193,72,invalid")

	got := ParseContactLabelIDs(extraBuffer)
	want := []int{72, 193}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseContactLabelIDs() = %v, want %v", got, want)
	}
}

func TestContactMatchTags(t *testing.T) {
	contact := &Contact{
		LabelIDs: []int{170, 176},
		Labels:   []string{"投资人", "AI圈"},
	}

	if !contact.MatchTags([]string{"投资人", "176"}, true) {
		t.Fatalf("expected contact to match all requested tags")
	}
	if !contact.MatchTags([]string{"Web3", "AI圈"}, false) {
		t.Fatalf("expected contact to match any requested tag")
	}
	if contact.MatchTags([]string{"Web3", "投资人"}, true) {
		t.Fatalf("expected contact not to match all requested tags")
	}
}
