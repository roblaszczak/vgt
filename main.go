package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/lmittmann/tint"
	"github.com/muesli/cancelreader"
)

//go:embed node_modules/plotly.js-dist/plotly.js
var plotly string

var debug bool

var dontPassOutput bool
var testDurationCutoff string
var testDurationCutoffDuration time.Duration
var printHTML bool
var keepRunning bool
var fromFile string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.BoolVar(&dontPassOutput, "dont-pass-output", false, "don't print output received to stdin")
	flag.BoolVar(&keepRunning, "keep-running", false, "keep browser running after page was opened")
	flag.BoolVar(&printHTML, "print-html", false, "print html to stdout instead of opening browser")
	flag.StringVar(&fromFile, "from-file", "", "read input from file instead of stdin")

	flag.StringVar(
		&testDurationCutoff,
		"duration-cutoff",
		"100µs",
		"threshold for test duration cutoff, under which tests are not shown in the chart",
	)
	flag.Parse()

	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}

	var err error
	testDurationCutoffDuration, err = time.ParseDuration(testDurationCutoff)
	if err != nil {
		panic(err)
	}

	slog.SetDefault(slog.New(
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      logLevel,
			TimeFormat: time.Kitchen,
		}),
	))

	r, cleanup, exitCode, done := newReader(ctx)
	if !done {
		return
	}
	defer cleanup()

	scanner := bufio.NewScanner(r)

	result := Parse(scanner)

	if checkClosing(ctx) {
		return
	}

	if printHTML {
		charts := generateCharts(result)
		html, err := render(result, charts, false)
		if err != nil {
			slog.Error("Error rendering html", "err", err)
			return
		}
		_, _ = os.Stdout.Write([]byte(html))
	} else {
		serveHTML(ctx, result)
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	if result.Failed {
		os.Exit(1)
	}
}

func newReader(ctx context.Context) (io.Reader, func(), int, bool) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		slog.Error("Error getting stdin stat", "err", err)
		return nil, nil, 0, false
	}

	isPipe := (fi.Mode() & os.ModeCharDevice) == 0
	readFromFile := fromFile != ""

	if isPipe && readFromFile {
		slog.Error("Can't read from file and stdin at the same time")
		return nil, nil, 0, false
	}

	if readFromFile {
		f, err := os.Open(fromFile)
		if err != nil {
			slog.Error("Error opening file", "err", err)
			return nil, nil, 0, false
		}

		return f, func() {
			_ = f.Close()
		}, 0, true
	}

	if isPipe {
		sr, err := cancelreader.NewReader(os.Stdin)
		if err != nil {
			slog.Error("Error creating cancel reader", "err", err)
			return nil, nil, 0, false
		}

		go func() {
			<-ctx.Done()
			sr.Cancel()
		}()

		return sr, func() {}, 0, true
	}

	r := bytes.NewBuffer([]byte{})

	command := append([]string{"go", "test", "-json"}, flag.Args()...)

	slog.Info("Running go test", "command", command)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(r, os.Stdout)
	cmd.Stderr = os.Stderr

	var exitCode int

	err = cmd.Run()
	var exitErr *exec.ExitError
	if err != nil {
		if errors.As(err, &exitErr) {
			// this is expected - tests failed
			slog.Info("Error running go test", "err", err)
			exitCode = exitErr.ExitCode()
		} else {
			slog.Error("Error running go test", "err", err)
			return nil, nil, 0, false
		}
	}

	go func() {
		<-ctx.Done()
		_ = cmd.Process.Kill()
	}()

	return r, func() {
		_, _ = cmd.Process.Wait()
	}, exitCode, true
}

func checkClosing(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		fmt.Println(
			`Process closed without input: you should pipe the output of your test command into this program. 
For example: go test -json ./... | vgt`,
		)
		return true
	default:
		return false
	}
}
