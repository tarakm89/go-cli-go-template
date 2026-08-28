// Package bootstrap is the process-level entry point: it owns the signal
// handling, the lifetime of telemetry and the mapping from an error to an exit
// code. main() does nothing but call it.
//
// Keeping this out of main() means the functional suite can run the entire
// application in-process, with fake adapters, and assert on the exit code the
// real binary would have returned.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"{{ cookiecutter.module_path }}/internal/adapter/inbound/cli"
	"{{ cookiecutter.module_path }}/internal/buildinfo"
)

// Options describes one run. The zero value runs the real application against
// the real os.Args.
type Options struct {
	// Args are the command line arguments, excluding the program name.
	// Nil means os.Args[1:].
	Args []string

	// Stdout and Stderr default to the process streams.
	Stdout io.Writer
	Stderr io.Writer

	// Adapters overrides the outbound adapters. Functional tests set this;
	// the binary never does.
	Adapters cli.Options
}

// Run executes the command tree and returns the process exit code. It never
// panics on a caller's behalf and never leaves telemetry unflushed.
func Run(ctx context.Context, opts Options) int {
	opts = withDefaults(opts)

	// A CI runner cancelling a step should still get a flushed trace, so the
	// signal cancels the command context while shutdown runs on its own.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	adapters := opts.Adapters
	adapters.Stdout, adapters.Stderr = opts.Stdout, opts.Stderr
	if adapters.Build.Name == "" {
		adapters.Build = buildinfo.Get()
	}

	root, shutdown := cli.NewRoot(adapters)
	root.SetArgs(opts.Args)

	runErr := root.ExecuteContext(ctx)

	if err := shutdown(ctx); err != nil {
		fmt.Fprintf(opts.Stderr, "warning: could not flush telemetry: %v\n", err)
	}

	return exitCode(runErr, opts.Stderr)
}

func withDefaults(opts Options) Options {
	if opts.Args == nil {
		opts.Args = os.Args[1:]
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return opts
}

// exitCode maps an error to a process status, printing it unless the command
// has already told the user what went wrong.
func exitCode(err error, stderr io.Writer) int {
	if err == nil {
		return cli.ExitOK
	}

	var silent interface{ Silent() bool }
	if !errors.As(err, &silent) || !silent.Silent() {
		fmt.Fprintf(stderr, "error: %v\n", err)
	}

	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}

	return cli.ExitError
}
