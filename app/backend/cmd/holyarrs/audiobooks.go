package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/outbound"
	"github.com/HolyArrs/holyarrs/app/backend/internal/core/webserver"
	"github.com/HolyArrs/holyarrs/app/backend/internal/downloads"
)

var audiobookHTTP = &http.Client{Timeout: 12 * time.Second}
var audiobookArchiveHTTP = &http.Client{
	Timeout: 20 * time.Second,
	Transport: &http.Transport{
		DialContext: outbound.PolicyDialer(outbound.PublicOnly).DialContext,
	},
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		u := r.URL
		if len(via) > 3 || u.Scheme != "https" || u.User != nil || u.Port() != "" ||
			(u.Host != "archive.org" && !strings.HasSuffix(u.Host, ".archive.org")) {
			return fmt.Errorf("unexpected Internet Archive redirect")
		}
		return nil
	},
}
var audiobookID = regexp.MustCompile(`^[0-9]{1,12}$`)
var archiveID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,199}$`)

type audiobookEdition struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Authors []struct {
		First string `json:"first_name"`
		Last  string `json:"last_name"`
	} `json:"authors"`
	Language string `json:"language"`
	Duration string `json:"totaltime"`
	Zip      string `json:"url_zip_file"`
}

func audiobookRoutes(m *http.ServeMux) {
	base := os.Getenv("HOLYARRS_AUDIOBOOK_QBIT_URL")
	key := os.Getenv("HOLYARRS_AUDIOBOOK_QBIT_KEY")
	qbit := downloads.NewQbittorrentKey(base, key, downloads.Client(base, true))
	m.HandleFunc("GET /api/v1/audiobooks/search", audiobookOwner(func(w http.ResponseWriter, r *http.Request) {
		if base == "" || key == "" {
			webserver.Fail(w, http.StatusServiceUnavailable, "The audiobook downloader is not configured.")
			return
		}
		title := strings.TrimSpace(r.URL.Query().Get("title"))
		if title == "" || len(title) > 200 {
			webserver.Fail(w, http.StatusBadRequest, "Enter a title of up to 200 bytes.")
			return
		}
		books, err := librivoxEditions(r, "", url.Values{"title": {title}, "limit": {"20"}})
		if err != nil {
			webserver.Fail(w, http.StatusBadGateway, "The LibriVox catalog is unavailable.")
			return
		}
		webserver.WriteJSON(w, http.StatusOK, books)
	}))
	m.HandleFunc("POST /api/v1/audiobooks/grab", audiobookOwner(func(w http.ResponseWriter, r *http.Request) {
		if base == "" || key == "" {
			webserver.Fail(w, http.StatusServiceUnavailable, "The audiobook downloader is not configured.")
			return
		}
		var input struct {
			ID string `json:"id"`
		}
		body := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
		if err := body.Decode(&input); err != nil || !audiobookID.MatchString(input.ID) || body.Decode(new(any)) != io.EOF {
			webserver.Fail(w, http.StatusBadRequest, "Choose a LibriVox edition.")
			return
		}
		books, err := librivoxEditions(r, input.ID, url.Values{})
		if err != nil {
			webserver.Fail(w, http.StatusBadGateway, "The LibriVox catalog is unavailable.")
			return
		}
		if len(books) != 1 || books[0].ID != input.ID {
			webserver.Fail(w, http.StatusNotFound, "This LibriVox edition could not be found.")
			return
		}
		link := audiobookTorrent(books[0].Zip)
		if link == "" {
			webserver.Fail(w, http.StatusUnprocessableEntity, "This edition has no supported Internet Archive torrent.")
			return
		}
		file, err := fetchAudiobookTorrent(r, link)
		if err != nil {
			webserver.Fail(w, http.StatusBadGateway, "The Internet Archive torrent could not be downloaded.")
			return
		}
		// Archive torrents can contain multiple copies of each chapter in
		// different formats. Let the owner select one set before starting.
		result, err := qbit.AddTorrents(r.Context(), downloads.AddBatch{
			Files: []downloads.TorrentUpload{file}, Tags: "holyarrs-librivox-" + input.ID, Paused: true,
		})
		if err != nil || result.Failed > 0 || result.Added+result.Pending+result.Duplicates == 0 {
			webserver.Fail(w, http.StatusBadGateway, "The torrent could not be added to the audiobook downloader.")
			return
		}
		webserver.WriteJSON(w, http.StatusOK, result)
	}))
}

func fetchAudiobookTorrent(r *http.Request, link string) (downloads.TorrentUpload, error) {
	var file downloads.TorrentUpload
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, link, nil)
	if err != nil {
		return file, err
	}
	resp, err := audiobookArchiveHTTP.Do(req)
	if err != nil {
		return file, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return file, fmt.Errorf("Internet Archive returned HTTP %d", resp.StatusCode)
	}
	const limit = 10 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return file, err
	}
	if len(b) == 0 || len(b) > limit {
		return file, fmt.Errorf("Internet Archive torrent has invalid size")
	}
	return downloads.TorrentUpload{Name: path.Base(req.URL.Path), Data: b}, nil
}

func audiobookOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, ok := webserver.From(r.Context())
		if !ok || caller.Role != webserver.RoleOwner || caller.Via != "session" {
			webserver.Fail(w, http.StatusForbidden, "Sign in as the owner to use Audiobooks.")
			return
		}
		next(w, r)
	}
}

func librivoxEditions(r *http.Request, id string, query url.Values) ([]audiobookEdition, error) {
	query.Set("format", "json")
	path := "https://librivox.org/api/feed/audiobooks/"
	if id != "" {
		path += "id/" + id + "/"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := audiobookHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []audiobookEdition{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LibriVox returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Books []audiobookEdition `json:"books"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if result.Books == nil {
		return []audiobookEdition{}, nil
	}
	return result.Books, nil
}

func audiobookTorrent(zip string) string {
	// Some LibriVox ZIP URLs contain literal spaces and omit URL escaping.
	u, err := url.Parse(zip)
	if err != nil || u.Scheme != "https" || u.Host != "archive.org" {
		return ""
	}
	prefix := "/compress/"
	if !strings.HasPrefix(u.Path, prefix) {
		return ""
	}
	id := strings.SplitN(strings.TrimPrefix(u.Path, prefix), "/", 2)[0]
	if !archiveID.MatchString(id) {
		return ""
	}
	return "https://archive.org/download/" + id + "/" + id + "_archive.torrent"
}
