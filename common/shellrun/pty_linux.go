//go:build linux

package shellrun

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

type console struct {
	pty *os.File
	cmd *exec.Cmd
}

func openTerm(cols, rows int) (terminal, error) {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	ptmx, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	if err := unix.IoctlSetPointerInt(int(ptmx.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	n, err := unix.IoctlGetInt(int(ptmx.Fd()), unix.TIOCGPTN)
	if err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		_ = ptmx.Close()
		return nil, err
	}
	_ = unix.IoctlSetWinsize(int(ptmx.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
	shell := "/bin/bash"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	// Ctty 是子进程里的描述符序号。标准输入就是从端，所以是 0。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = ptmx.Close()
		return nil, err
	}
	_ = slave.Close()
	// 主端留在本进程。代理进程退出时内核关掉主端，前台进程组会收到挂断。
	return &console{pty: ptmx, cmd: cmd}, nil
}

func (c *console) Read(p []byte) (int, error)  { return c.pty.Read(p) }
func (c *console) Write(p []byte) (int, error) { return c.pty.Write(p) }

func (c *console) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return nil
	}
	return unix.IoctlSetWinsize(int(c.pty.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
}

func (c *console) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		// 结束整组前台进程，避免 vim、ping 在 shell 退出后留下来。
		_ = syscall.Kill(-c.cmd.Process.Pid, syscall.SIGKILL)
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	if c.pty != nil {
		return c.pty.Close()
	}
	return nil
}
