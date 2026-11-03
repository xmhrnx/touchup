package media

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A stand-in ffplay that, like a package manager's shim, runs the real
// player as a child and waits for it.
func TestStopEndsWholePlayer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	shim := filepath.Join(dir, "ffplay")
	script := "#!/bin/sh\nsleep 60 &\necho $! > " + pidFile + "\nwait\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var p Player
	if err := p.Play(Tools{FFplay: shim}, "song.wav"); err != nil {
		t.Fatal(err)
	}
	var pid int
	for deadline := time.Now().Add(5 * time.Second); pid == 0; time.Sleep(20 * time.Millisecond) {
		data, _ := os.ReadFile(pidFile)
		pid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		if time.Now().After(deadline) {
			t.Fatal("the stand-in never started its child")
		}
	}
	if p.Playing() != "song.wav" {
		t.Errorf("playing %q", p.Playing())
	}
	p.Stop()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		// Signal 0 checks the process exists; a zombie still counts, so
		// look at its state too.
		alive := syscall.Kill(pid, 0) == nil
		if alive {
			if out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output(); err == nil && strings.HasPrefix(strings.TrimSpace(string(out)), "Z") {
				alive = false
			}
		}
		if !alive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the player's child kept playing after Stop")
		}
	}
	if p.Playing() != "" {
		t.Errorf("still playing %q after Stop", p.Playing())
	}
}
