package main

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func waitForSignals() os.Signal {
	sink := make(chan os.Signal, 1)
	defer close(sink)

	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGSTOP,
		syscall.SIGHUP,
		syscall.SIGQUIT,
	}

	if runtime.GOOS == "windows" {
		signals = []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT,
		}
	}

	// wait for signal
	signal.Notify(sink, signals...)
	// reset the watched signals
	defer signal.Ignore(signals...)

	return <-sink
}
