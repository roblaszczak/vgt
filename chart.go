package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func generateCharts(pr ParseResult) []PlotlyChart {
	var charts []PlotlyChart

	testNames := pr.TestNamesOrderedByStart()

	for _, tn := range testNames {
		ch := PlotlyChart{
			Type:         "bar",
			Orientation:  "h",
			Hoverinfo:    "text",
			Textposition: "inside",
		}

		pause, hasPause := pr.TestPauses.ByTestName(tn)
		run, hasRun := pr.TestRuns.ByTestName(tn)

		if !hasRun {
			slog.Debug("Test was not executed", "test", tn)
			continue
		}

		packageNameParts := strings.Split(tn.Package, "/")

		var packageName string
		if len(packageNameParts) != 0 {
			packageName = packageNameParts[len(packageNameParts)-1]
		} else {
			slog.Warn("Package name is empty", "test", tn.Package)
		}

		packageNameFull := fmt.Sprintf("%s.%s", packageName, tn.TestName)
		y := packageNameFull

		if !run.Passed {
			y += " (failed)"
		}

		if hasPause {
			startAfter := pause.Start.Sub(pr.Start)
			duration := pause.Duration()

			slog.Debug("Test was paused", "startAfter", startAfter, "duration", duration, "test", tn)

			ch.Add(
				fmt.Sprintf("%s PAUSE (%s)", packageNameFull, duration.Round(time.Millisecond).String()),
				y,
				startAfter,
				duration,
				"rgba(108,122,137,1)",
			)
		}

		{
			startAfter := run.Start.Sub(pr.Start)
			duration := run.Duration()

			slog.Debug("Test was executed", "startAfter", startAfter, "duration", duration, "test", tn)

			ch.Add(
				fmt.Sprintf("%s RUN (%s)", packageNameFull, duration.Round(time.Millisecond)),
				y,
				startAfter,
				duration,
				durationToRgb(run, pr.MaxDuration),
			)
		}

		slog.Debug("PlotlyChart", "chart", ch)

		charts = append(charts, ch)
	}

	return charts
}
