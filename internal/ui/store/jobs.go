package store

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/mehran9675/giode"

	"github.com/mehran9675/touchup/internal/engine"
	"github.com/mehran9675/touchup/internal/media"
	"github.com/mehran9675/touchup/internal/pipeline"
	"github.com/mehran9675/touchup/internal/tr"
)

// JobStatus is where a job is in its life.
type JobStatus int

const (
	JobQueued JobStatus = iota
	JobRunning
	JobDone
	JobFailed
	JobCanceled
)

// Job is one file being upscaled.
type Job struct {
	ID     int
	Kind   media.Kind
	Input  string
	Output string
	Name   string
	Label  string // engine, model and scale
	Info   media.Info

	Status   JobStatus
	Update   pipeline.Update
	Log      LiveLog // what the engine has printed so far
	Pace     Pace
	Err      string
	ErrLog   string // what the engine printed last, when it failed
	Created  time.Time
	Started  time.Time
	Finished time.Time

	opts   pipeline.Options
	cancel context.CancelFunc
}

// Ended reports whether the job finished, failed or was canceled.
func (j *Job) Ended() bool { return j.Status >= JobDone }

// Enqueue adds the open file to the queue with the current selection.
func (s *Store) Enqueue() {
	sess := s.Session
	if sess == nil || sess.Loading || sess.Err != "" {
		return
	}
	opts, sel := s.options(sess.Kind)
	if err := sel.problem(sess.Kind); err != nil {
		s.fail(err)
		return
	}
	var ext string
	switch sess.Kind {
	case media.Image:
		ext = "." + s.Settings.ImageFormat
	case media.Video:
		ext = media.VideoExt(s.Settings.VideoCodec)
	case media.Audio:
		ext = "." + s.Settings.AudioFormat
	}
	out := pipeline.OutputPath(sess.Path, s.Settings, sel.SizeLabel(), ext, func(p string) bool {
		for _, j := range s.Jobs {
			if !j.Ended() && j.Output == p {
				return true
			}
		}
		return false
	})
	job := &Job{
		ID: s.id(), Kind: sess.Kind, Input: sess.Path, Output: out,
		Name: sess.Name, Label: sel.Label(), Info: sess.Info,
		Created: time.Now(), opts: opts,
	}
	s.Jobs = append(s.Jobs, job)
	s.Toasts.ShowAction(giode.ToastInfo, tr.T("Added {file} to the queue", "file", sess.Name), tr.T("View"), func() {
		s.Navigate(PageQueue)
	})
	s.pump()
}

// pump starts the next queued job when nothing is running. Jobs run one
// at a time: engines use the whole GPU.
func (s *Store) pump() {
	for _, j := range s.Jobs {
		if j.Status == JobRunning {
			return
		}
	}
	for _, j := range s.Jobs {
		if j.Status == JobQueued {
			s.start(j)
			return
		}
	}
}

func (s *Store) start(j *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	j.Status, j.cancel, j.Started = JobRunning, cancel, time.Now()
	j.Update = pipeline.Update{Stage: tr.T("Starting"), Frac: 0}
	j.Log.reset()
	j.Pace.reset()
	report := func(u pipeline.Update) {
		s.post(func() {
			if u.Line != "" {
				j.Log.add(u.Line)
			}
			if u.Frac >= 0 && u.Frac > j.Update.Frac {
				j.Pace.add(u.Frac)
			}
			if u.Frac < 0 {
				u.Frac = j.Update.Frac
			}
			if u.Line == "" {
				u.Line = j.Update.Line
			}
			j.Update = u
		})
	}
	go func() {
		var err error
		switch j.Kind {
		case media.Image:
			err = pipeline.Image(ctx, j.opts, j.Input, j.Output, report)
		case media.Video:
			err = pipeline.Video(ctx, j.opts, j.Input, j.Output, j.Info, report)
		case media.Audio:
			err = pipeline.Audio(ctx, j.opts, j.Input, j.Output, j.Info.Duration, report)
		}
		s.post(func() {
			j.Finished, j.cancel = time.Now(), nil
			switch {
			case errors.Is(err, context.Canceled):
				j.Status = JobCanceled
			case err != nil:
				j.Status, j.Err, j.ErrLog = JobFailed, err.Error(), engine.LogOf(err)
				s.Toasts.Show(giode.ToastError, tr.T("{file} failed: {reason}", "file", j.Name, "reason", err.Error()))
			default:
				j.Status = JobDone
				j.Update.Frac = 1
				s.Toasts.ShowAction(giode.ToastSuccess, tr.T("{file} is ready", "file", filepath.Base(j.Output)), tr.T("Show"), func() {
					s.Reveal(j.Output)
				})
			}
			s.pump()
		})
	}()
}

// CancelJob stops a running job or drops a queued one.
func (s *Store) CancelJob(j *Job) {
	switch j.Status {
	case JobQueued:
		j.Status, j.Finished = JobCanceled, time.Now()
	case JobRunning:
		if j.cancel != nil {
			j.cancel()
		}
	}
}

// RetryJob queues a failed or canceled job again.
func (s *Store) RetryJob(j *Job) {
	if !j.Ended() || j.Status == JobDone {
		return
	}
	j.Status, j.Err, j.ErrLog, j.Update = JobQueued, "", "", pipeline.Update{}
	s.pump()
}

// ClearFinished removes ended jobs from the list.
func (s *Store) ClearFinished() {
	var keep []*Job
	for _, j := range s.Jobs {
		if !j.Ended() {
			keep = append(keep, j)
		}
	}
	s.Jobs = keep
}

// ActiveJobs counts queued and running jobs.
func (s *Store) ActiveJobs() int {
	n := 0
	for _, j := range s.Jobs {
		if !j.Ended() {
			n++
		}
	}
	return n
}

// Reveal shows a file in the file manager.
func (s *Store) Reveal(path string) {
	go func() {
		s.fail(s.App.Share(context.Background(), giode.ShareFile{Path: path}))
	}()
}

// Shutdown stops everything running in the background, so no engine or
// FFmpeg process outlives the app.
func (s *Store) Shutdown() {
	for _, j := range s.Jobs {
		if j.cancel != nil {
			j.cancel()
		}
	}
	for _, in := range s.Installs {
		if in.Running() {
			in.cancel()
		}
	}
	for _, in := range s.ModelInstalls {
		if in.Running() {
			in.cancel()
		}
	}
	s.Close()
}
