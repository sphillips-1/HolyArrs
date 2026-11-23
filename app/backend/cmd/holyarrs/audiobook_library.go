package main

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/webserver"
)

var audiobookItemID = regexp.MustCompile(`^[A-Za-z0-9-]{1,80}$`)

func audiobookCover(client *http.Client, base, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !audiobookItemID.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		if base == "" || key == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		target := strings.TrimRight(base, "/") + "/internal/audiobook-cover?id=" + id
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			http.NotFound(w, r)
			return
		}
		kind := resp.Header.Get("Content-Type")
		if resp.StatusCode != http.StatusOK || (kind != "image/jpeg" && kind != "image/png" && kind != "image/webp") {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		const maxCover = 512 << 10
		cover, err := io.ReadAll(io.LimitReader(resp.Body, maxCover+1))
		if err != nil || len(cover) == 0 || len(cover) > maxCover {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", kind)
		webserver.CacheHeaders(w, "private, max-age=300", false)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(cover)
	}
}

type audiobookLibraryPage struct {
	Items []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Author   string `json:"author"`
		Narrator string `json:"narrator"`
	} `json:"items"`
	Total int `json:"total"`
	Limit int `json:"limit"`
}

func audiobookLibrary(client *http.Client, base, key string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if base == "" || key == "" {
			webserver.Fail(w, http.StatusServiceUnavailable, "The audiobook library is not configured.")
			return
		}
		page := r.URL.Query().Get("page")
		if page == "" {
			page = "0"
		}
		n, err := strconv.Atoi(page)
		if err != nil || n < 0 || n > 10000 {
			webserver.Fail(w, http.StatusBadRequest, "Choose a valid audiobook catalog page.")
			return
		}
		target := strings.TrimRight(base, "/") + "/internal/audiobook-library?page=" + strconv.Itoa(n)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
		if err != nil {
			webserver.Fail(w, http.StatusServiceUnavailable, "The audiobook library is not configured.")
			return
		}
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := client.Do(req)
		if err != nil {
			webserver.Fail(w, http.StatusBadGateway, "Audiobookshelf is unavailable.")
			return
		}
		defer resp.Body.Close()
		pageView := audiobookLibraryPage{}
		if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&pageView) != nil ||
			pageView.Items == nil || pageView.Limit <= 0 || pageView.Total < 0 {
			webserver.Fail(w, http.StatusBadGateway, "Audiobookshelf is unavailable.")
			return
		}
		webserver.WriteJSONNoStore(w, http.StatusOK, pageView)
	}
}
