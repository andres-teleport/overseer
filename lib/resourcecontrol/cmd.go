package resourcecontrol

import (
	"errors"
	"io"
	"os"
	"os/exec"
)

type Cmd struct {
	*exec.Cmd
}

// Start wraps exec.Cmd.Start and adds a signalling pipe to catch errors earlier
func (c *Cmd) Start() error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	c.ExtraFiles = []*os.File{w}

	if err = c.Cmd.Start(); err != nil {
		return err
	}

	if err = w.Close(); err != nil {
		return err
	}

	out, err := io.ReadAll(r)
	if err != nil {
		return err
	} else if len(out) > 0 {
		return errors.New(string(out))
	}

	return nil
}
