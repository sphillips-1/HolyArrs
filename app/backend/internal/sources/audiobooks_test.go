package sources

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/database"
	domain "github.com/HolyArrs/holyarrs/app/backend/internal/core/model"
)

func TestAudiobookSourceSearch(t *testing.T) {
	indexer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("t") == "caps" {
			_, _ = io.WriteString(w, `<caps><searching><search available="yes"/><movie-search available="yes"/></searching></caps>`)
			return
		}
		if q.Get("t") != "search" || q.Get("q") != "Pride and Prejudice" || q.Get("cat") != "3030" {
			t.Errorf("unexpected audiobook Source query: %s", q.Encode())
		}
		_, _ = io.WriteString(w, `<rss><channel><item><title>Pride and Prejudice</title><guid>fixture</guid>
<enclosure url="https://example.org/book.torrent" length="500" type="application/x-bittorrent"/></item></channel></rss>`)
	}))
	defer indexer.Close()

	f := seed(t, indexer.URL)
	ctx := context.Background()
	src := &domain.Source{
		Name: "Books", Protocol: "torrent", Format: "torznab", URL: indexer.URL, AllowLocal: true,
		UseFor: domain.SourceUse{HandSearch: true}, Kinds: []string{"audiobook"}, State: "enabled", Priority: 25,
	}
	if err := f.s.Write(ctx, func(tx *sql.Tx) error {
		_, err := database.Insert(tx, "sources", src, domain.System)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	searcher := &Searcher{Store: f.s}
	list, err := searcher.AudiobookSources(ctx)
	if err != nil || len(list) != 1 || list[0].ID != src.ID {
		t.Fatalf("eligible sources = %+v, error = %v", list, err)
	}
	items, err := searcher.SearchAudiobook(ctx, list[0], "Pride and Prejudice")
	if err != nil || len(items) != 1 || items[0].GUID != "fixture" {
		t.Fatalf("search results = %+v, error = %v", items, err)
	}
}
