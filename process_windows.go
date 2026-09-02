//go:build windows

package main

import "os/exec"

// group is what a process group costs on Windows: nothing. The command bar that
// runs tasks is not offered there — run.sh needs a shell the platform does not
// ship — so these exist to be compiled, not to be called.
func group(*exec.Cmd) {}

func interrupt(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	return command.Process.Kill()
}

func finish(*exec.Cmd) {}
