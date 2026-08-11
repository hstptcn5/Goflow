//go:build windows

package main

import (
	"debug/pe"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

const (
	createNewConsole    = 0x00000010
	ctrlCEvent          = 0
	attachParentProcess = ^uint32(0)
)

var (
	kernel32Proc             = syscall.NewLazyDLL("kernel32.dll")
	attachConsoleProc        = kernel32Proc.NewProc("AttachConsole")
	freeConsoleProc          = kernel32Proc.NewProc("FreeConsole")
	generateConsoleCtrlEvent = kernel32Proc.NewProc("GenerateConsoleCtrlEvent")
	setConsoleCtrlHandler    = kernel32Proc.NewProc("SetConsoleCtrlHandler")
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewConsole,
		HideWindow:    true,
	}
}

func interruptProcess(pid int) (func(), error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_, _, _ = freeConsoleProc.Call()
	attached, _, attachErr := attachConsoleProc.Call(uintptr(uint32(pid)))
	if attached == 0 {
		_, _, _ = attachConsoleProc.Call(uintptr(attachParentProcess))
		return nil, fmt.Errorf("attach to appliance console: %v", attachErr)
	}
	ignored, _, ignoreErr := setConsoleCtrlHandler.Call(0, 1)
	if ignored == 0 {
		_, _, _ = freeConsoleProc.Call()
		_, _, _ = attachConsoleProc.Call(uintptr(attachParentProcess))
		return nil, fmt.Errorf("protect smoke harness from console interrupt: %v", ignoreErr)
	}
	sent, _, sendErr := generateConsoleCtrlEvent.Call(ctrlCEvent, 0)
	_, _, _ = freeConsoleProc.Call()
	_, _, _ = attachConsoleProc.Call(uintptr(attachParentProcess))
	if sent == 0 {
		_, _, _ = setConsoleCtrlHandler.Call(0, 0)
		return nil, fmt.Errorf("send console interrupt to appliance: %v", sendErr)
	}
	return func() {
		// Console control delivery is asynchronous. Keep the harness protected
		// until the child has exited and the event can no longer reach this process.
		time.Sleep(250 * time.Millisecond)
		_, _, _ = setConsoleCtrlHandler.Call(0, 0)
	}, nil
}

func verifyPEAMD64(path string) error {
	file, err := pe.Open(path)
	if err != nil {
		return fmt.Errorf("open appliance PE runtime: %w", err)
	}
	defer file.Close()
	if file.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		return fmt.Errorf("appliance runtime is not a Windows AMD64 PE executable")
	}
	return nil
}
