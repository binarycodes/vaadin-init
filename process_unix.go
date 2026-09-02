//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

// group puts a task in a process group of its own.
//
// run.sh is a script that runs Maven, which runs a compiler and sometimes a
// server: signalling the script alone leaves all of that behind, still holding
// the port the next run needs. A group of its own is what makes the whole tree
// stoppable by one signal.
func group(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// interrupt asks a task's whole process group to finish, the way ctrl+c in a
// terminal would — the negative pid is the group.
func interrupt(command *exec.Cmd) error {
	return signal(command, syscall.SIGINT)
}

// finish makes sure nothing is left of a task that was stopped.
//
// The polite signal is not always enough. A shell script's background children
// ignore an interrupt when there is no job control, so the script goes and they
// stay — still holding the pipe their output was read from, and still holding
// whatever port they had. What that costs is the terminal, which this tool
// cannot give back while a task has hold of it.
func finish(command *exec.Cmd) {
	_ = signal(command, syscall.SIGKILL)
}

func signal(command *exec.Cmd, sig syscall.Signal) error {
	if command.Process == nil {
		return nil
	}
	return syscall.Kill(-command.Process.Pid, sig)
}
