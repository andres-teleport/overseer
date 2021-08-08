package supervisor

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"

	"github.com/andres-teleport/overseer/lib/multipipe"
	"github.com/andres-teleport/overseer/lib/resourcecontrol"
)

var (
	ErrUnknownJobID = errors.New("unknown job ID")
	ErrJobFinished  = errors.New("job was already finished")
)

const (
	StatusStarted = iota
	StatusDone
	StatusStopped
)

type Status struct {
	Status   int
	ExitCode int
}

// TODO: add option to set the environment variables
type Job struct {
	cmd    *exec.Cmd
	status Status
	stdout *multipipe.MultiPipe
	stderr *multipipe.MultiPipe
}

type Supervisor struct {
	mu        *sync.Mutex
	processes map[string]*Job
}

// NewSupervisor returns a Supervisor struct that will allow starting, stopping
// and operating with jobs
func NewSupervisor() *Supervisor {
	return &Supervisor{
		mu:        &sync.Mutex{},
		processes: make(map[string]*Job),
	}
}

func (s *Supervisor) jobApplyFn(id string, jobFn func(*Job)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if j, ok := s.processes[id]; ok {
		jobFn(j)
		return nil
	}

	return ErrUnknownJobID
}

// StartJob runs the given command and arguments, enforcing resource controls.
// Returns a UUID to identify the job or an error on failure. cmd must be an
// absolute path.
func (s *Supervisor) StartJob(cmd string, args ...string) (string, error) {
	job := &Job{
		cmd: resourcecontrol.Command(
			// TODO: make configurable
			resourcecontrol.ResourceLimits{
				CPUMax:    "10000 100000", // 10 %
				MemMax:    "128M",
				IOMaxRbps: "5000000",
				IOMaxWbps: "5000000",
			},
			cmd,
			args...,
		),
		status: Status{
			Status: StatusStarted,
		},
		stdout: multipipe.NewMultiPipe(),
		stderr: multipipe.NewMultiPipe(),
	}
	job.cmd.Stdout = job.stdout
	job.cmd.Stderr = job.stderr

	uuid, err := ioutil.ReadFile("/proc/sys/kernel/random/uuid")
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(uuid))

	s.mu.Lock()
	s.processes[id] = job
	s.mu.Unlock()

	if err := job.cmd.Start(); err != nil {
		return "", err
	}

	go func() {
		state, err := job.cmd.Process.Wait()
		job.stdout.CloseWithError(err)
		job.stderr.CloseWithError(err)

		s.mu.Lock()
		if job.status.Status != StatusStopped {
			job.status.Status = StatusDone
		}

		job.status.ExitCode = state.ExitCode()
		s.mu.Unlock()
	}()

	return id, nil
}

// StopJob kills the job with the given ID, unless it has already finished,
// returning an error in that case
func (s *Supervisor) StopJob(id string) error {
	var innerErr error

	if err := s.jobApplyFn(id, func(j *Job) {
		if j.status.Status != StatusStarted {
			innerErr = ErrJobFinished
			return
		}

		j.status = Status{
			Status: StatusStopped,
		}

		innerErr = j.cmd.Process.Kill()
	}); err != nil {
		return err
	}

	return innerErr
}

// JobStatus returns the status of the job with the given ID, or an error if the
// job was not found
func (s *Supervisor) JobStatus(id string) (status Status, err error) {
	err = s.jobApplyFn(id, func(j *Job) {
		status = j.status
	})

	return
}

// JobStdOut returns an io.Reader corresponding to the standard output of the
// job with the given ID, or an error if the job was not found
func (s *Supervisor) JobStdOut(id string) (rd *multipipe.Reader, err error) {
	err = s.jobApplyFn(id, func(j *Job) {
		rd = j.stdout.NewReader()
	})

	return
}

// JobStdErr returns an io.Reader corresponding to the standard error of the job
// with the given ID, or an error if the job was not found
func (s *Supervisor) JobStdErr(id string) (rd *multipipe.Reader, err error) {
	err = s.jobApplyFn(id, func(j *Job) {
		rd = j.stderr.NewReader()
	})

	return
}
