package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A stand-in server in Python: it copies its input to its output, notes
// each start in a file, and serves jobs like the SeedVR2 launcher.
const fakeServer = `import json, os, shutil, sys
starts, argv = sys.argv[1], sys.argv[2:]
open(starts, "a").write("start\n")
while True:
    if argv[0] == "fail":
        print("Error: asked to fail", flush=True)
        code = 1
    else:
        print("50%", flush=True)
        shutil.copy(argv[0], argv[1])
        code = 0
    print(f"TOUCHUP-SERVE-DONE {code}", flush=True)
    if code or os.environ.get("TOUCHUP_SERVE") != "1":
        sys.exit(code)
    line = sys.stdin.readline()
    if not line:
        break
    argv = json.loads(line)[2:]
`

func TestServe(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("needs python3")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "server.py")
	starts := filepath.Join(dir, "starts.txt")
	if err := os.WriteFile(script, []byte(fakeServer), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{ID: "served", Name: "Served", Media: []string{"image"}, Input: "image", Serve: true,
		Executable: map[string]string{"*": python}, Args: []string{script, starts, "{input}", "{output}"},
		Install: InstallSpec{Type: "none"}, OutputExt: ".txt"}
	t.Setenv("TOUCHUP_HOME", t.TempDir())
	defer StopServers()

	run := func(name string) error {
		in := filepath.Join(dir, name+".in")
		os.WriteFile(in, []byte(name), 0o644)
		var progress float64
		j := Job{Input: in, Output: filepath.Join(dir, name+".txt"), OutputDir: dir, Progress: func(f float64) { progress = f }}
		got, err := m.Run(context.Background(), j)
		if err != nil {
			return err
		}
		if data, _ := os.ReadFile(got); string(data) != name {
			t.Errorf("%s: got %q", name, data)
		}
		if progress != 0.5 {
			t.Errorf("%s: progress %v, want 0.5", name, progress)
		}
		return nil
	}
	count := func() int {
		data, _ := os.ReadFile(starts)
		return strings.Count(string(data), "start")
	}
	for _, name := range []string{"one", "two", "three"} {
		if err := run(name); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(); n != 1 {
		t.Errorf("started %d times for three jobs, want once", n)
	}

	// A failed job reports its error and the next job starts afresh.
	j := Job{Input: "fail", Output: filepath.Join(dir, "x.txt"), OutputDir: dir}
	if _, err := m.Run(context.Background(), j); err == nil || !strings.Contains(err.Error(), "asked to fail") {
		t.Errorf("got %v, want the job's error", err)
	}
	if err := run("four"); err != nil {
		t.Fatal(err)
	}
	if n := count(); n != 2 {
		t.Errorf("started %d times, want twice after a failure", n)
	}

	// Another engine running stops this one's server.
	other := &Manifest{ID: "other", Name: "Other", Media: []string{"image"}, Input: "image",
		Executable: map[string]string{"*": python}, Args: []string{"-c", "pass"}, Install: InstallSpec{Type: "none"}, OutputExt: ".txt"}
	other.Run(context.Background(), Job{Output: filepath.Join(dir, "four.txt"), OutputDir: dir})
	deadline := time.Now().Add(5 * time.Second)
	for {
		serversMu.Lock()
		n := len(servers)
		serversMu.Unlock()
		if n == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the server kept running when another engine ran")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
