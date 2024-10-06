package main

import (
	"bufio"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
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

	logLevel := slog.LevelWarn
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

	var r io.Reader

	if fromFile == "" {
		sr, err := cancelreader.NewReader(os.Stdin)
		if err != nil {
			slog.Error("Error creating cancel reader", "err", err)
			return
		}

		go func() {
			<-ctx.Done()
			sr.Cancel()
		}()

		r = sr
	} else {
		f, err := os.Open(fromFile)
		if err != nil {
			slog.Error("Error opening file", "err", err)
			return
		}
		defer func() {
			_ = f.Close()
		}()

		r = f
	}

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
