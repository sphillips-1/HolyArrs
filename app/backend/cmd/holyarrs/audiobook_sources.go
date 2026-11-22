package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"

	domain "github.com/HolyArrs/holyarrs/app/backend/internal/core/model"
	"github.com/HolyArrs/holyarrs/app/backend/internal/core/webserver"
	"github.com/HolyArrs/holyarrs/app/backend/internal/downloads"
	"github.com/HolyArrs/holyarrs/app/backend/internal/sources"
)

type audiobookReleaseView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Size    int64  `json:"size"`
	Seeders *int   `json:"seeders,omitempty"`
}

func audiobookReleaseID(item sources.Item) string {
	key := item.GUID
	if key == "" {
		key = item.Link
	}
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func audiobookSourceItems(w http.ResponseWriter, r *http.Request, searcher *sources.Searcher, id int64, title string) (domain.Source, []sources.Item, bool) {
	if searcher == nil || id <= 0 {
		webserver.Fail(w, http.StatusNotFound, "Choose an enabled audiobook Source.")
		return domain.Source{}, nil, false
	}
	all, err := searcher.AudiobookSources(r.Context())
	if err != nil {
		webserver.Internal(w, "audiobook sources", err)
		return domain.Source{}, nil, false
	}
	index := slices.IndexFunc(all, func(src domain.Source) bool { return src.ID == id })
	if index < 0 {
		webserver.Fail(w, http.StatusNotFound, "This audiobook Source is unavailable.")
		return domain.Source{}, nil, false
	}
	src := all[index]
	items, err := searcher.SearchAudiobook(r.Context(), src, title)
	if err != nil {
		webserver.Fail(w, http.StatusBadGateway, "The audiobook Source could not complete the search.")
		return src, nil, false
	}
	return src, items, true
}

func searchAudiobookSource(w http.ResponseWriter, r *http.Request, searcher *sources.Searcher, id int64, title string) {
	_, items, ok := audiobookSourceItems(w, r, searcher, id, title)
	if !ok {
		return
	}
	results := []audiobookReleaseView{}
	seen := map[string]bool{}
	for _, item := range items {
		id := audiobookReleaseID(item)
		if item.Link == "" || seen[id] {
			continue
		}
		results = append(results, audiobookReleaseView{ID: id, Title: item.Title, Size: item.Size, Seeders: item.Seeders})
		seen[id] = true
		if len(results) == 100 {
			break
		}
	}
	webserver.WriteJSON(w, http.StatusOK, results)
}

func grabAudiobookSource(w http.ResponseWriter, r *http.Request, searcher *sources.Searcher, qbit downloads.TorrentAdder, id int64, title, release string) {
	title = strings.TrimSpace(title)
	if _, err := hex.DecodeString(release); title == "" || len(title) > 200 || len(release) != 64 || err != nil {
		webserver.Fail(w, http.StatusBadRequest, "Choose an audiobook release from the search results.")
		return
	}
	src, items, ok := audiobookSourceItems(w, r, searcher, id, title)
	if !ok {
		return
	}
	for _, item := range items {
		if item.Link == "" || audiobookReleaseID(item) != release {
			continue
		}
		var transport http.RoundTripper
		if src.ProxyURL != "" {
			cfg, err := sources.ConfigFor(r.Context(), src, searcher.Secrets)
			if err != nil {
				webserver.Fail(w, http.StatusBadGateway, "The audiobook Source proxy is unavailable.")
				return
			}
			transport = sources.Transport(cfg)
		}
		batch, err := downloads.TorrentBatchFromSource(r.Context(), item.Link, src.URL, src.AllowLocal, transport)
		if err != nil {
			webserver.Fail(w, http.StatusBadGateway, "The audiobook release could not be fetched.")
			return
		}
		queueAudiobook(w, r, qbit, batch)
		return
	}
	webserver.Fail(w, http.StatusConflict, "This release changed or is unavailable. Search again.")
}
