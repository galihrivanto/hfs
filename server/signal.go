package server

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func waitForSignals() chan os.Signal {
	sink := make(chan os.Signal, 1)

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

	return sink
}
