package sources

import (
	"context"
	"slices"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/database"
	domain "github.com/HolyArrs/holyarrs/app/backend/internal/core/model"
)

// AudiobookSources lists enabled torrent indexers explicitly configured for
// audiobook hand searches. Existing movie and TV source defaults stay intact.
func (s *Searcher) AudiobookSources(ctx context.Context) ([]domain.Source, error) {
	all, err := database.Select[domain.Source](ctx, s.Store.Read(), "sources", "state = 'enabled' AND protocol = 'torrent' ORDER BY priority, name")
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(all, func(src domain.Source) bool {
		return src.Format != "torznab" || !src.UseFor.HandSearch || !slices.Contains(src.Kinds, "audiobook")
	}), nil
}

// SearchAudiobook uses the configured Source client, including its credentials,
// proxy, network policy and shared query limits.
func (s *Searcher) SearchAudiobook(ctx context.Context, src domain.Source, title string) ([]Item, error) {
	client, err := s.client(ctx, src)
	if err != nil {
		return nil, err
	}
	categories := src.Categories["audiobook"]
	if len(categories) == 0 {
		categories = []int{3030}
	}
	return client.Search(ctx, Query{Title: title, Audiobook: true, Categories: categories})
}
