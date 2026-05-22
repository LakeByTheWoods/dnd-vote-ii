package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLoadConfigReadsEnvironmentWithLocalDefaults(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg, err := LoadConfigFromLookup(func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatalf("LoadConfigFromLookup returned error: %v", err)
		}

		if cfg.Host != "0.0.0.0" {
			t.Fatalf("Host = %q, want local bind default", cfg.Host)
		}
		if cfg.Port != "8000" {
			t.Fatalf("Port = %q, want 8000", cfg.Port)
		}
		if cfg.DataDir != "data" {
			t.Fatalf("DataDir = %q, want data", cfg.DataDir)
		}
		if cfg.DatabasePath != filepath.Join("data", "dnd_vote.db") {
			t.Fatalf("DatabasePath = %q, want default data database", cfg.DatabasePath)
		}
		if cfg.AdminSecret != "" {
			t.Fatalf("AdminSecret = %q, want empty local default", cfg.AdminSecret)
		}
		if cfg.MaxRequestBodyBytes != 8192 {
			t.Fatalf("MaxRequestBodyBytes = %d, want 8192", cfg.MaxRequestBodyBytes)
		}
	})

	t.Run("environment overrides", func(t *testing.T) {
		env := map[string]string{
			"DND_VOTE_HOST":                   "127.0.0.1",
			"DND_VOTE_PORT":                   "9000",
			"DND_VOTE_DATA_DIR":               filepath.Join("tmp", "data"),
			"DND_VOTE_DB_PATH":                filepath.Join("tmp", "custom.db"),
			"DND_VOTE_ADMIN_SECRET":           "secret",
			"DND_VOTE_MAX_REQUEST_BODY_BYTES": "1024",
		}

		cfg, err := LoadConfigFromLookup(func(key string) (string, bool) {
			value, ok := env[key]
			return value, ok
		})
		if err != nil {
			t.Fatalf("LoadConfigFromLookup returned error: %v", err)
		}

		if cfg.Host != "127.0.0.1" || cfg.Port != "9000" {
			t.Fatalf("address config = %s:%s, want 127.0.0.1:9000", cfg.Host, cfg.Port)
		}
		if cfg.DataDir != filepath.Join("tmp", "data") {
			t.Fatalf("DataDir = %q", cfg.DataDir)
		}
		if cfg.DatabasePath != filepath.Join("tmp", "custom.db") {
			t.Fatalf("DatabasePath = %q", cfg.DatabasePath)
		}
		if cfg.AdminSecret != "secret" {
			t.Fatalf("AdminSecret = %q", cfg.AdminSecret)
		}
		if cfg.MaxRequestBodyBytes != 1024 {
			t.Fatalf("MaxRequestBodyBytes = %d", cfg.MaxRequestBodyBytes)
		}
	})
}

func TestOpenStoreCreatesFreshSQLiteSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "dnd_vote.db")

	store, err := OpenStore(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("querying sqlite schema: %v", err)
	}
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scanning schema row: %v", err)
		}
		tables[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("schema rows error: %v", err)
	}

	for _, table := range []string{"polls", "poll_dates", "votes", "vote_rankings", "vote_unavailability"} {
		if !tables[table] {
			t.Fatalf("schema is missing table %s; got %#v", table, tables)
		}
	}
}

func TestRouterServesExistingStaticAppShell(t *testing.T) {
	handler := testRouter(t, 8192)

	for _, test := range []struct {
		path     string
		contains string
	}{
		{path: "/", contains: "Dungeons & Dates"},
		{path: "/index.html", contains: "New Poll"},
		{path: "/vote.html", contains: "Submit Vote"},
		{path: "/results.html", contains: "Results"},
		{path: "/styles.css", contains: "page-shell"},
		{path: "/shared.js", contains: "window.DndVoteApp"},
		{path: "/host.js", contains: "createPoll"},
		{path: "/vote.js", contains: "saveVote"},
		{path: "/results.js", contains: "getResults"},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s returned status %d", test.path, response.Code)
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("GET %s did not include %q", test.path, test.contains)
			}
		})
	}
}

func TestJSONAPIRequestsEnforceMaximumBodySize(t *testing.T) {
	handler := testRouter(t, 32)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/polls",
		strings.NewReader(`{"title":"`+strings.Repeat("x", 64)+`"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}

	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response was not JSON: %v", err)
	}
	if payload["error"] != "Request body must be 32 bytes or fewer." {
		t.Fatalf("error payload = %#v", payload)
	}
}

func TestNewHTTPServerSetsProductionTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Host = "127.0.0.1"
	cfg.Port = "0"

	server := NewHTTPServer(cfg, http.NewServeMux())

	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want 127.0.0.1:0", server.Addr)
	}
	for name, value := range map[string]time.Duration{
		"ReadHeaderTimeout": server.ReadHeaderTimeout,
		"ReadTimeout":       server.ReadTimeout,
		"WriteTimeout":      server.WriteTimeout,
		"IdleTimeout":       server.IdleTimeout,
	} {
		if value <= 0 {
			t.Fatalf("%s = %s, want a configured timeout", name, value)
		}
	}
}

func testRouter(t *testing.T, maxBodyBytes int64) http.Handler {
	t.Helper()

	store, err := OpenStore(context.Background(), filepath.Join(t.TempDir(), "dnd_vote.db"))
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	cfg := DefaultConfig()
	cfg.StaticDir = repoRoot(t)
	cfg.MaxRequestBodyBytes = maxBodyBytes

	return NewRouter(cfg, store)
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
