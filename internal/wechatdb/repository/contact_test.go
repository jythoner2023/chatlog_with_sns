package repository

import (
	"context"
	"testing"

	"github.com/sjzar/chatlog/internal/model"
)

func TestGetContactsFiltersByTags(t *testing.T) {
	repo := &Repository{
		contactCache: map[string]*model.Contact{
			"alice": {UserName: "alice", Labels: []string{"投资人", "AI圈"}, LabelIDs: []int{170, 186}},
			"bob":   {UserName: "bob", Labels: []string{"AI圈"}, LabelIDs: []int{186}},
			"carol": {UserName: "carol", Labels: []string{"Web3"}, LabelIDs: []int{197}},
		},
		contactList: []string{"alice", "bob", "carol"},
	}

	contacts, err := repo.GetContacts(context.Background(), "", "投资人,AI圈", "all", 0, 0)
	if err != nil {
		t.Fatalf("GetContacts(all tags) error = %v", err)
	}
	if len(contacts) != 1 || contacts[0].UserName != "alice" {
		t.Fatalf("GetContacts(all tags) = %#v, want only alice", contacts)
	}

	contacts, err = repo.GetContacts(context.Background(), "", "投资人,AI圈", "any", 0, 0)
	if err != nil {
		t.Fatalf("GetContacts(any tags) error = %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("GetContacts(any tags) len = %d, want 2", len(contacts))
	}
}

func TestGetContactsCombinesKeywordTagsAndPagination(t *testing.T) {
	repo := &Repository{
		contactCache: map[string]*model.Contact{
			"alice": {UserName: "alice", Labels: []string{"投资人", "AI圈"}, LabelIDs: []int{170, 186}},
			"bob":   {UserName: "bob", Labels: []string{"AI圈"}, LabelIDs: []int{186}},
		},
		contactList: []string{"alice", "bob"},
	}

	contacts, err := repo.GetContacts(context.Background(), "alice", "投资人", "all", 1, 0)
	if err != nil {
		t.Fatalf("GetContacts(keyword+tags) error = %v", err)
	}
	if len(contacts) != 1 || contacts[0].UserName != "alice" {
		t.Fatalf("GetContacts(keyword+tags) = %#v, want only alice", contacts)
	}

	contacts, err = repo.GetContacts(context.Background(), "", "AI圈", "any", 1, 1)
	if err != nil {
		t.Fatalf("GetContacts(pagination) error = %v", err)
	}
	if len(contacts) != 1 || contacts[0].UserName != "bob" {
		t.Fatalf("GetContacts(pagination) = %#v, want only bob", contacts)
	}
}
