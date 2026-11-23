package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/HolyArrs/holyarrs/app/backend/internal/core/webserver"
)

func TestAudiobookTorrent(t *testing.T) {
	const want = "https://archive.org/download/pride_and_prejudice_librivox/pride_and_prejudice_librivox_archive.torrent"
	for _, tc := range []struct{ url, want string }{
		{"https://archive.org/compress/pride_and_prejudice_librivox/formats=64KBPS MP3&file=/pride_and_prejudice_librivox.zip", want},
		{"https://archive.org.example.com/compress/book", ""},
		{"https://archive.org/compress/%2e%2e", ""},
		{"https://archive.org/compress/book%3Fother", ""},
	} {
		if got := audiobookTorrent(tc.url); got != tc.want {
			t.Errorf("audiobookTorrent(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestAudiobookLibrary(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/audiobook-library" || r.URL.Query().Get("page") != "2" ||
			r.Header.Get("Authorization") != "Bearer fixture" {
			t.Errorf("unexpected catalog gateway request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"sample","title":"Example","path":"/private"}],"total":101,"limit":50}`)
	}))
	defer gateway.Close()
	handler := audiobookOwner(audiobookLibrary(gateway.Client(), gateway.URL, "fixture"))
	for _, tc := range []struct {
		role, page string
		status     int
	}{
		{webserver.RoleMember, "2", http.StatusForbidden},
		{webserver.RoleOwner, "-1", http.StatusBadRequest},
		{webserver.RoleOwner, "2", http.StatusOK},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/library?page="+tc.page, nil)
		caller := webserver.Caller{Role: tc.role, Via: "session"}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r.WithContext(webserver.ContextWith(r.Context(), caller)))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "/private") {
			t.Fatalf("unexpected catalog response for %s: %d %s", tc.role, w.Code, w.Body)
		}
	}
}

func TestAudiobookCover(t *testing.T) {
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/audiobook-cover" || r.URL.Query().Get("id") != "book-1" ||
			r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("unexpected cover gateway request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = io.WriteString(w, "cover fixture")
	}))
	defer gateway.Close()
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/audiobooks/cover/{id}", audiobookOwner(audiobookCover(gateway.Client(), gateway.URL, "fixture")))
	for _, tc := range []struct {
		role, via, id string
		status        int
	}{
		{webserver.RoleMember, "session", "book-1", http.StatusForbidden},
		{webserver.RoleOwner, "api_key", "book-1", http.StatusForbidden},
		{webserver.RoleOwner, "session", "book%2F1", http.StatusNotFound},
		{webserver.RoleOwner, "session", "book-1", http.StatusOK},
	} {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/audiobooks/cover/"+tc.id, nil)
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r.WithContext(webserver.ContextWith(r.Context(), webserver.Caller{Role: tc.role, Via: tc.via})))
		if w.Code != tc.status {
			t.Fatalf("got HTTP %d, want %d", w.Code, tc.status)
		}
		if tc.status == http.StatusOK && (w.Header().Get("Content-Type") != "image/jpeg" || w.Body.String() != "cover fixture") {
			t.Fatal("unexpected cover response")
		}
	}
}

func TestAudiobookRoutes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		caller     webserver.Caller
		method     string
		path       string
		body       string
		configured bool
		status     int
	}{
		{"member", webserver.Caller{Role: webserver.RoleMember, Via: "session"}, "GET", "/search", "", false, 403},
		{"key", webserver.Caller{Role: webserver.RoleOwner, Via: "api_key"}, "GET", "/search", "", false, 403},
		{"unconfigured", webserver.Caller{Role: webserver.RoleOwner, Via: "session"}, "GET", "/search", "", false, 503},
		{"missing title", webserver.Caller{Role: webserver.RoleOwner, Via: "session"}, "GET", "/search", "", true, 400},
		{"external release", webserver.Caller{Role: webserver.RoleOwner, Via: "session"}, "POST", "/grab", `{"id":"https://example.com/x.torrent"}`, true, 400},
		{"extra body", webserver.Caller{Role: webserver.RoleOwner, Via: "session"}, "POST", "/grab", `{"id":"253"}{}`, true, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_URL", "")
			t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_KEY", "")
			if tc.configured {
				t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_URL", "http://localhost:8080")
				t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_KEY", "test")
			}
			m := http.NewServeMux()
			audiobookRoutes(m, nil)
			r := httptest.NewRequest(tc.method, "/api/v1/audiobooks"+tc.path, strings.NewReader(tc.body))
			w := httptest.NewRecorder()
			m.ServeHTTP(w, r.WithContext(webserver.ContextWith(r.Context(), tc.caller)))
			if w.Code != tc.status {
				t.Fatalf("got HTTP %d (%s), want %d", w.Code, w.Body, tc.status)
			}
		})
	}
}

func TestAudiobookPublicDomainSmoke(t *testing.T) {
	if os.Getenv("HOLYARRS_AUDIOBOOK_LIVE_SMOKE") != "1" {
		t.Skip("set HOLYARRS_AUDIOBOOK_LIVE_SMOKE=1 to query LibriVox and the Internet Archive")
	}
	uploaded := make(chan bool, 1)
	qbit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			_, _ = io.WriteString(w, "[]")
		case "/api/v2/torrents/add":
			torrent, _, err := r.FormFile("torrents")
			if err != nil {
				http.Error(w, "missing torrent", http.StatusBadRequest)
				return
			}
			defer torrent.Close()
			uploaded <- r.FormValue("stopped") == "true" && r.FormValue("tags") == "holyarrs-librivox-253"
			_, _ = io.WriteString(w, "Ok.")
		default:
			http.NotFound(w, r)
		}
	}))
	defer qbit.Close()
	t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_URL", qbit.URL)
	t.Setenv("HOLYARRS_AUDIOBOOK_QBIT_KEY", "test")
	m := http.NewServeMux()
	audiobookRoutes(m, nil)
	owner := webserver.Caller{Role: webserver.RoleOwner, Via: "session"}
	run := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/audiobooks"+path, strings.NewReader(body))
		w := httptest.NewRecorder()
		m.ServeHTTP(w, r.WithContext(webserver.ContextWith(r.Context(), owner)))
		if w.Code != http.StatusOK {
			t.Fatalf("%s %s returned %d: %s", method, path, w.Code, w.Body)
		}
		return w
	}
	search := run("GET", "/search?title=pride+and+prejudice", "")
	var editions []audiobookEdition
	if err := json.Unmarshal(search.Body.Bytes(), &editions); err != nil || len(editions) == 0 {
		t.Fatalf("expected LibriVox search results: %v", err)
	}
	run("POST", "/grab", `{"id":"253"}`)
	select {
	case paused := <-uploaded:
		if !paused {
			t.Fatal("the archive torrent was not queued paused with its LibriVox edition tag")
		}
	default:
		t.Fatal("the downloader did not receive the archive torrent")
	}
}
