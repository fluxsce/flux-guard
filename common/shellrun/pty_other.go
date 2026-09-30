//go:build !windows && !linux

package shellrun

import (
	"io"
	"os/exec"
)

type console struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	outR io.ReadCloser
	outW io.WriteCloser
}

func openTerm(cols, rows int) (terminal, error) {
	cmd := exec.Command("/bin/sh")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	outR, outW := io.Pipe()
	cmd.Stdout = outW
	cmd.Stderr = outW
	if err := cmd.Start(); err != nil {
		_ = in.Close()
		_ = outW.Close()
		return nil, err
	}
	return &console{cmd: cmd, in: in, outR: outR, outW: outW}, nil
}

func (c *console) Read(p []byte) (int, error)  { return c.outR.Read(p) }
func (c *console) Write(p []byte) (int, error) { return c.in.Write(p) }
func (c *console) Resize(int, int) error       { return nil }

func (c *console) Close() error {
	_ = c.in.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.outW.Close()
	_ = c.cmd.Wait()
	return nil
}
