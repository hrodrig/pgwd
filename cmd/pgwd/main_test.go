// Black-box tests for the pgwd binary. Build the binary once in TestMain, then exec it.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "pgwd-test-*")
	if err != nil {
		panic("cannot create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	testBinary = filepath.Join(dir, "pgwd")
	if runtime.GOOS == "windows" {
		testBinary += ".exe"
	}

	// Repo root: main_test.go is in cmd/pgwd/, so three levels up to module root.
	_, self, _, _ := runtime.Caller(0)
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(self)))

	cmd := exec.Command("go", "build", "-o", testBinary, "./cmd/pgwd")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("go build failed: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

func runBinary(args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command(testBinary, args...)
	var outb, errb strings.Builder
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	stdout = outb.String()
	stderr = errb.String()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout, stderr, exitCode
}

func TestMain_PrintSampleConfig(t *testing.T) {
	stdout, stderr, code := runBinary("--print-sample-config")
	if code != 0 {
		t.Errorf("pgwd --print-sample-config: exit code %d, want 0", code)
	}
	if stdout == "" || !strings.Contains(stdout, "client:") {
		t.Errorf("pgwd --print-sample-config: stdout %q does not contain sample config", stdout)
	}
	if stderr != "" {
		t.Errorf("pgwd --print-sample-config: expected empty stderr, got %q", stderr)
	}
}

func TestMain_Version(t *testing.T) {
	stdout, _, code := runBinary("-version")
	if code != 0 {
		t.Errorf("pgwd -version: exit code %d, want 0", code)
	}
	if !strings.Contains(stdout, "pgwd") {
		t.Errorf("pgwd -version: stdout %q does not contain 'pgwd'", stdout)
	}
}

func TestMain_VersionLong(t *testing.T) {
	stdout, _, code := runBinary("--version")
	if code != 0 {
		t.Errorf("pgwd --version: exit code %d, want 0", code)
	}
	if !strings.Contains(stdout, "pgwd") {
		t.Errorf("pgwd --version: stdout %q does not contain 'pgwd'", stdout)
	}
}

func TestMain_Help(t *testing.T) {
	stdout, stderr, code := runBinary("-h")
	if code != 0 {
		t.Errorf("pgwd -h: exit code %d, want 0", code)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "pgwd") {
		t.Errorf("pgwd -h: output %q does not contain 'pgwd'", combined)
	}
}

func TestMain_MissingClient(t *testing.T) {
	// Config with databases url but no client: should fail validation
	configPath := filepath.Join(t.TempDir(), "pgwd.conf")
	if err := os.WriteFile(configPath, []byte("databases:\n  - url: postgres://localhost/test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runBinary("-config", configPath)
	if code == 0 {
		t.Errorf("pgwd with missing client: want non-zero exit, got 0")
	}
	if !strings.Contains(stderr, "client") {
		t.Errorf("pgwd with missing client: stderr %q should mention client", stderr)
	}
}

func TestMain_LegacyDBRemoved(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "pgwd.conf")
	if err := os.WriteFile(configPath, []byte("client: x\ndb:\n  url: postgres://localhost/test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runBinary("-config", configPath, "-dry-run")
	if code == 0 {
		t.Errorf("legacy db: want non-zero exit, got 0")
	}
	if !strings.Contains(stderr, "db:") {
		t.Errorf("stderr %q should mention removed db:", stderr)
	}
}

func TestMain_MissingDBURL(t *testing.T) {
	// Config with client but no db url (single-DB mode): should fail
	configPath := filepath.Join(t.TempDir(), "pgwd.conf")
	if err := os.WriteFile(configPath, []byte("client: mymonitor\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runBinary("-config", configPath)
	if code == 0 {
		t.Errorf("pgwd with missing db url: want non-zero exit, got 0")
	}
	if !strings.Contains(stderr, "database") && !strings.Contains(stderr, "db") {
		t.Errorf("pgwd with missing db url: stderr %q should mention database/db", stderr)
	}
}

func TestMain_ConnectFailureExit2(t *testing.T) {
	_, stderr, code := runBinary(
		"-client", "exit2-test",
		"-db-url", "postgres://127.0.0.1:1/nope?sslmode=disable",
		"-dry-run",
	)
	if code != 2 {
		t.Errorf("connect failure: exit %d, want 2; stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "postgres connect failed") {
		t.Errorf("stderr %q should mention connect failed", stderr)
	}
}
