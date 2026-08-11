//go:build !windows

package main

import (
	"fmt"
	"os/exec"
)

func configureProcess(_ *exec.Cmd) {}

func interruptProcess(_ int) (func(), error) {
	return nil, fmt.Errorf("native Windows process control is unavailable")
}

func verifyPEAMD64(_ string) error {
	return fmt.Errorf("native Windows PE verification is unavailable")
}
