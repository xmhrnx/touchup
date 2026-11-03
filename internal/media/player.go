package media

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/mehran9675/touchup/internal/proc"
)

// ErrNoPlayer is returned when ffplay isn't installed.
var ErrNoPlayer = errors.New("ffplay was not found: install FFmpeg with ffplay to listen to previews")

// Player plays one audio file at a time through ffplay.
type Player struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	stop    context.CancelFunc
	playing string
	// OnChange is called from a background goroutine when playback
	// starts or stops, with the file playing or "".
	OnChange func(playing string)
}

// Play stops whatever is playing and starts path.
func (p *Player) Play(tools Tools, path string) error {
	if tools.FFplay == "" {
		return ErrNoPlayer
	}
	p.Stop()
	// Cancelling the context ends ffplay and everything it started: on
	// Windows "ffplay" is often a package manager's shim that runs the
	// real one as a child, which killing the shim alone leaves playing.
	ctx, stop := context.WithCancel(context.Background())
	cmd := proc.Command(ctx, tools.FFplay, "-nodisp", "-autoexit", "-loglevel", "quiet", path)
	if err := cmd.Start(); err != nil {
		stop()
		return err
	}
	p.mu.Lock()
	p.cmd, p.stop, p.playing = cmd, stop, path
	p.mu.Unlock()
	p.notify(path)
	go func() {
		cmd.Wait()
		stop()
		p.mu.Lock()
		current := p.cmd == cmd
		if current {
			p.cmd, p.stop, p.playing = nil, nil, ""
		}
		p.mu.Unlock()
		if current {
			p.notify("")
		}
	}()
	return nil
}

// Stop ends playback.
func (p *Player) Stop() {
	p.mu.Lock()
	cmd, stop := p.cmd, p.stop
	p.cmd, p.stop, p.playing = nil, nil, ""
	p.mu.Unlock()
	if cmd != nil {
		stop()
		p.notify("")
	}
}

// Playing returns the file playing, or "".
func (p *Player) Playing() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *Player) notify(path string) {
	if p.OnChange != nil {
		p.OnChange(path)
	}
}
