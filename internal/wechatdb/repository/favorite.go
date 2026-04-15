package repository

import (
	"context"

	"github.com/sjzar/chatlog/internal/model"
)

func (r *Repository) GetFavorites(ctx context.Context, favType string, keyword string, limit, offset int) ([]*model.FavoriteItem, error) {
	items, err := r.ds.GetFavorites(ctx, favType, keyword, limit, offset)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if contact := r.getFullContact(item.FromUser); contact != nil {
			if displayName := contact.DisplayName(); displayName != "" {
				item.FromDisplayName = displayName
			}
		}
		if contact := r.getFullContact(item.SourceChat); contact != nil {
			if displayName := contact.DisplayName(); displayName != "" {
				item.SourceChatDisplay = displayName
			}
		}
	}

	return items, nil
}
