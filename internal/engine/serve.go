package engine

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mehran9675/touchup/internal/proc"
)

// Engines that load big models, like SeedVR2, spend much of a short job
// starting Python and loading the model. They can stay running instead
// (Manifest.Serve): the first job starts a server, later jobs with the
// same settings are sent to it, and it stops after serveIdle without
// work, when settings change, when another engine runs, after a failed
// or cancelled job, and when the app quits.

// serveIdle is how long a server waits for another job before quitting
// and giving back its memory.
const serveIdle = 10 * time.Minute

// serveDone starts the line a server prints after each job, followed by
// the job's exit code.
const serveDone = "TOUCHUP-SERVE-DONE"

type server struct {
	engine string
	// key is the command without what changes from job to job; a job
	// with another key needs a new server.
	key    string
	cmd    *exec.Cmd
	cancel context.CancelFunc
	quit   chan struct{} // closed by stop
	once   sync.Once
	stdin  io.WriteCloser
	lines  chan string // what it prints; closed when it exits
	busy   bool
	idle   *time.Timer
	exited chan struct{}
}

var (
	serversMu sync.Mutex
	servers   = map[string]*server{} // by engine ID
)

// StopServers stops every engine server, for when the app quits.
func StopServers() { stopServers("") }

// stopServers stops the servers of engines other than except.
func stopServers(except string) {
	serversMu.Lock()
	defer serversMu.Unlock()
	for id, s := range servers {
		if id != except {
			s.stop()
			delete(servers, id)
		}
	}
}

// serveKey is the command without the job's own files and sizes, which
// a server takes job by job; the rest (model, settings, GPU,
// environment) is fixed when it starts.
func (m *Manifest) serveKey(exe, dir string, j Job, env []string) string {
	j.Input, j.Output, j.InputDir, j.OutputDir = "", "", "", ""
	j.InWidth, j.InHeight, j.OutWidth, j.OutHeight, j.Scale = 0, 0, 0, 0, 0
	var b strings.Builder
	for _, part := range [][]string{{exe, dir}, m.Command(j), env} {
		for _, a := range part {
			b.WriteString(a)
			b.WriteByte(0)
		}
		b.WriteByte(1)
	}
	return b.String()
}

// runServed runs a job on the engine's server, starting one if there's
// none for these settings. It returns false without running anything
// when the server is busy with another job, which then runs on its own.
func (m *Manifest) runServed(ctx context.Context, j Job, exe string, args, env []string, dir string, handle func(string)) (bool, error) {
	key := m.serveKey(exe, dir, j, env)
	serversMu.Lock()
	s := servers[m.ID]
	if s != nil && s.busy {
		serversMu.Unlock()
		return false, nil
	}
	fresh := false
	if s != nil && (s.key != key || s.dead()) {
		s.stop()
		delete(servers, m.ID)
		s = nil
	}
	if s == nil {
		var err error
		if s, err = startServer(m.ID, key, exe, args, env, dir); err != nil {
			serversMu.Unlock()
			handle("Error: " + err.Error())
			return true, err
		}
		servers[m.ID] = s
		fresh = true
	}
	s.busy = true
	if s.idle != nil {
		s.idle.Stop()
	}
	serversMu.Unlock()

	if !fresh {
		// Its first job came with its arguments; later ones are sent.
		s.drain()
		line, _ := json.Marshal(args)
		if _, err := s.stdin.Write(append(line, '\n')); err != nil {
			s.retire()
			return true, err
		}
	}
	err := s.wait(ctx, handle)
	if err != nil {
		// A failed job can leave the models in a bad state, and a
		// cancelled one is still running: start again next time.
		s.retire()
		return true, err
	}
	serversMu.Lock()
	s.busy = false
	s.idle = time.AfterFunc(serveIdle, func() {
		serversMu.Lock()
		defer serversMu.Unlock()
		if servers[s.engine] == s && !s.busy {
			s.stop()
			delete(servers, s.engine)
		}
	})
	serversMu.Unlock()
	return true, nil
}

func startServer(engine, key, exe string, args, env []string, dir string) (*server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := proc.Command(ctx, exe, args...)
	cmd.Dir, cmd.Env = dir, append(env, "TOUCHUP_SERVE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	s := &server{engine: engine, key: key, cmd: cmd, cancel: cancel, stdin: stdin, quit: make(chan struct{}),
		lines: make(chan string, 1024), exited: make(chan struct{})}
	go func() {
		proc.Lines(pr, func(line string) {
			select {
			case s.lines <- line:
			case <-s.quit: // nobody's listening any more
			}
		})
		close(s.lines)
	}()
	go func() {
		cmd.Wait()
		pw.Close()
		cancel()
		close(s.exited)
	}()
	return s, nil
}

// wait passes the server's output to handle until the job ends.
func (s *server) wait(ctx context.Context, handle func(string)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-s.lines:
			if !ok {
				<-s.exited
				return &exec.ExitError{ProcessState: s.cmd.ProcessState}
			}
			if rest, found := strings.CutPrefix(strings.TrimSpace(line), serveDone); found {
				code, err := strconv.Atoi(strings.TrimSpace(rest))
				if err != nil || code != 0 {
					return errServedJob
				}
				return nil
			}
			handle(line)
		}
	}
}

var errServedJob = jobFailed{}

type jobFailed struct{}

func (jobFailed) Error() string { return "the job failed" }

// drain drops output left over from earlier jobs.
func (s *server) drain() {
	for {
		select {
		case _, ok := <-s.lines:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

// stop kills the server and everything it started.
func (s *server) stop() {
	s.once.Do(func() {
		close(s.quit)
		s.cancel()
	})
}

func (s *server) dead() bool {
	select {
	case <-s.exited:
		return true
	default:
		return false
	}
}

// retire stops the server and forgets it.
func (s *server) retire() {
	serversMu.Lock()
	defer serversMu.Unlock()
	s.stop()
	if servers[s.engine] == s {
		delete(servers, s.engine)
	}
}
