package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryCutoverUsesGoBackendOnly(t *testing.T) {
	root := repoRoot(t)

	railwayConfig := readTextFile(t, filepath.Join(root, "railway.toml"))
	if !strings.Contains(railwayConfig, `startCommand = "go run ./cmd/dnd-vote"`) {
		t.Fatalf("railway.toml must start the Go server directly; got:\n%s", railwayConfig)
	}

	verifyScript := readTextFile(t, filepath.Join(root, "scripts", "verify.py"))
	for _, required := range []string{`"test", "./..."`, `"--check"`} {
		if !strings.Contains(verifyScript, required) {
			t.Fatalf("scripts/verify.py does not include %q", required)
		}
	}
	for _, removed := range []string{"py_compile", "unittest", "server.py", "main.py"} {
		if strings.Contains(verifyScript, removed) {
			t.Fatalf("scripts/verify.py still references Python server verification via %q", removed)
		}
	}

	for _, path := range []string{
		".python-version",
		"main.py",
		"server.py",
		"requirements.txt",
		filepath.Join("tests", "test_server_behavior.py"),
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed after the Go cutover", path)
		}
	}
}

func TestReadmeDocumentsGoServerOperations(t *testing.T) {
	readme := readTextFile(t, filepath.Join(repoRoot(t), "README.txt"))
	if strings.Contains(strings.ToLower(readme), "migrat") {
		t.Fatalf("README.txt should describe the Go server as current, not as a migration:\n%s", readme)
	}

	for _, required := range []string{
		"supported backend is the Go server",
		"go run ./cmd/dnd-vote",
		"python scripts\\verify.py",
		"go test ./...",
		"node --check",
		"data/dnd_vote.db",
		"DND_VOTE_DATA_DIR",
		"DND_VOTE_DB_PATH",
		"Railway volume",
		"manage",
		"X-DnD-Vote-Management-Token",
		"DND_VOTE_ADMIN_SECRET",
		"X-DnD-Vote-Admin-Secret",
	} {
		if !strings.Contains(readme, required) {
			t.Fatalf("README.txt does not document %q", required)
		}
	}
}

func readTextFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
