//go:build windows

package shellrun

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// console 是一块 Windows 伪控制台。程序以为自己连着真屏幕，所以整屏刷新能画出来。
type console struct {
	in   windows.Handle
	out  windows.Handle
	hPC  windows.Handle
	proc windows.Handle
	job  windows.Handle
	attr *windows.ProcThreadAttributeListContainer
}

func openTerm(cols, rows int) (terminal, error) {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		closeHandles(inR, inW)
		return nil, err
	}
	c := &console{in: inW, out: outR}
	err := windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, inR, outW, 0, &c.hPC)
	if err != nil {
		closeHandles(inR, inW, outR, outW)
		return nil, err
	}
	c.attr, err = windows.NewProcThreadAttributeList(1)
	if err != nil {
		closeHandles(inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	// 伪控制台属性要的是句柄值本身。传地址会让子进程退出码变成 0xC0000142。
	handle := *(*unsafe.Pointer)(unsafe.Pointer(&c.hPC))
	if err := c.attr.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, handle, unsafe.Sizeof(c.hPC)); err != nil {
		closeHandles(inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	path := os.Getenv("ComSpec")
	if path == "" {
		path = `C:\Windows\System32\cmd.exe`
	}
	app, err := windows.UTF16PtrFromString(path)
	if err != nil {
		closeHandles(inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	cmdLine, err := windows.UTF16FromString(windows.ComposeCommandLine([]string{path}))
	if err != nil {
		closeHandles(inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	si := &windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			// 清掉从父进程继承的标准句柄，输出才会进伪控制台，而不是测试进程自己的屏幕。
			Flags: windows.STARTF_USESTDHANDLES,
		},
		ProcThreadAttributeList: c.attr.List(),
	}
	pi := new(windows.ProcessInformation)
	err = windows.CreateProcess(
		app, &cmdLine[0], nil, nil, false,
		windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_SUSPENDED,
		nil, nil, &si.StartupInfo, pi,
	)
	if err != nil {
		closeHandles(inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	if job, err := createKillJob(pi.Process); err == nil {
		c.job = job
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		closeHandles(pi.Process, pi.Thread, inR, inW, outR, outW)
		_ = c.Close()
		return nil, err
	}
	_ = windows.CloseHandle(pi.Thread)
	// 伪控制台已经复制了这两端。父进程留着的话，子进程退出后读端不会收到结束。
	closeHandles(inR, outW)
	c.proc = pi.Process
	return c, nil
}

func createKillJob(process windows.Handle) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

func closeHandles(handles ...windows.Handle) {
	for _, h := range handles {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
	}
}

func (c *console) Read(p []byte) (int, error) {
	if c.out == 0 {
		return 0, os.ErrClosed
	}
	var n uint32
	err := windows.ReadFile(c.out, p, &n, nil)
	return int(n), err
}

func (c *console) Write(p []byte) (int, error) {
	if c.in == 0 {
		return 0, os.ErrClosed
	}
	var n uint32
	err := windows.WriteFile(c.in, p, &n, nil)
	return int(n), err
}

func (c *console) Resize(cols, rows int) error {
	if c == nil || c.hPC == 0 || cols < 1 || rows < 1 {
		return nil
	}
	return windows.ResizePseudoConsole(c.hPC, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (c *console) Close() error {
	if c.hPC != 0 {
		windows.ClosePseudoConsole(c.hPC)
		c.hPC = 0
	}
	if c.in != 0 {
		_ = windows.CloseHandle(c.in)
		c.in = 0
	}
	if c.out != 0 {
		_ = windows.CancelIoEx(c.out, nil)
		_ = windows.CloseHandle(c.out)
		c.out = 0
	}
	if c.job != 0 {
		_ = windows.CloseHandle(c.job)
		c.job = 0
	}
	if c.proc != 0 {
		_ = windows.TerminateProcess(c.proc, 1)
		_ = windows.CloseHandle(c.proc)
		c.proc = 0
	}
	if c.attr != nil {
		c.attr.Delete()
		c.attr = nil
	}
	return nil
}
