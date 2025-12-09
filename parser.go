package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"
)

type testOutput struct {
	Time    time.Time `json:"Time"`
	Action  action    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
}

type action string

const (
	actionRun   action = "run"
	actionPause action = "pause"

	actionPass action = "pass"
	actionCont action = "cont"
	actionFail action = "fail"
	actionSkip action = "skip"
)

func (t testOutput) IsZero() bool {
	return t == testOutput{}
}

type TestName struct {
	Package  string
	TestName string
}

func (t TestName) String() string {
	return fmt.Sprintf("%s/%s", t.Package, t.TestName)
}

type TestExecution struct {
	Test TestName

	Start time.Time
	End   time.Time

	Passed bool
}

func (t TestExecution) Duration() time.Duration {
	if t.Start.IsZero() || t.End.IsZero() {
		return 0
	}

	return t.End.Sub(t.Start)
}

type TestExecutions map[TestName]TestExecution

func (t TestExecutions) MarshalJSON() ([]byte, error) {
	m := map[string]TestExecution{}

	for k, v := range t {
		m[k.String()] = v
	}

	return json.Marshal(m)
}

func (t TestExecutions) Update(testName TestName, updateFn func(TestExecution) TestExecution) {
	if _, ok := t[testName]; !ok {
		t[testName] = TestExecution{
			Test: testName,
		}
	}
	t[testName] = updateFn(t[testName])
}

func (t TestExecutions) ByTestName(testName TestName) (TestExecution, bool) {
	te, ok := t[testName]
	return te, ok
}

func (t TestExecutions) AsSlice() []TestExecution {
	slice := make([]TestExecution, 0, len(t))

	for _, execution := range t {
		slice = append(slice, execution)
	}
	return slice
}

type ParseResult struct {
	TestPauses TestExecutions
	TestRuns   TestExecutions

	Start time.Time
	End   time.Time

	MaxDuration time.Duration

	Failed bool
}

func (p ParseResult) TestNamesOrderedByStart() []TestName {
	allExecutions := append(p.TestPauses.AsSlice(), p.TestRuns.AsSlice()...)

	sort.Slice(allExecutions, func(i, j int) bool {
		return allExecutions[i].Start.Before(allExecutions[j].Start)
	})

	testNames := make([]TestName, 0, len(p.TestRuns))
	uniqTestNames := make(map[TestName]struct{}, len(p.TestRuns))

	for _, execution := range allExecutions {
		if _, ok := uniqTestNames[execution.Test]; !ok {
			testNames = append(testNames, execution.Test)
			uniqTestNames[execution.Test] = struct{}{}
		}
	}

	return testNames
}

func Parse(scanner *bufio.Scanner) ParseResult {
	testRuns := make(TestExecutions)
	testPauses := make(TestExecutions)

	start := time.Time{}
	end := time.Time{}

	maxDuration := time.Duration(0)

	failed := false

	i := 0

	for scanner.Scan() {
		s := scanner.Text()
		i++

		if !dontPassOutput {
			_, _ = fmt.Fprintln(os.Stderr, s)
		}

		if s == "" {
			continue
		}

		var out testOutput
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			slog.Debug("failed to unmarshal", "line", i, "error", err)
			continue
		}
		if out.IsZero() {
			slog.Debug("zero value", "line", i)
			continue
		}

		if out.Action == actionFail {
			failed = true
		}

		if !out.Time.IsZero() {
			if start.IsZero() || out.Time.Before(start) {
				start = out.Time
			}
			if end.IsZero() || out.Time.After(end) {
				end = out.Time
			}
		}

		tn := TestName{
			Package:  out.Package,
			TestName: out.Test,
		}

		switch out.Action {
		case actionPause:
			testPauses.Update(tn, func(te TestExecution) TestExecution {
				te.Start = out.Time
				return te
			})
		case actionCont:
			testPauses.Update(tn, func(te TestExecution) TestExecution {
				te.End = out.Time
				return te
			})
			testRuns.Update(tn, func(te TestExecution) TestExecution {
				te.Start = out.Time
				return te
			})

		case actionRun:
			testRuns.Update(tn, func(te TestExecution) TestExecution {
				te.Start = out.Time
				return te
			})
		case actionPass, actionFail, actionSkip:
			testRuns.Update(tn, func(te TestExecution) TestExecution {
				te.End = out.Time
				te.Passed = out.Action != actionFail
				return te
			})
		}
	}

	for test, execution := range testPauses {
		if execution.Duration() == 0 {
			delete(testPauses, test)
			slog.Debug("removed invalid test pause", "test", test)
			continue
		}
		if execution.Duration() <= testDurationCutoffDuration {
			delete(testPauses, test)
			slog.Debug("removed test pause below threshold", "test", test, "duration", execution.Duration())
			continue
		}
		if execution.Test == (TestName{}) {
			delete(testPauses, test)
			slog.Debug("removed test pause with empty test name", "test", test)
			continue
		}

		slog.Debug(
			"parsed test pause",
			"test", execution.Test,
			"paused_from", execution.Start,
			"to", execution.End,
			"for", execution.Duration(),
		)
	}

	for test, execution := range testRuns {
		if execution.Duration() == 0 {
			delete(testRuns, test)
			slog.Debug("removed invalid test run", "test", test)
			continue
		}

		if execution.Duration() <= testDurationCutoffDuration {
			delete(testRuns, test)
			slog.Debug("removed test run below threshold", "test", test, "duration", execution.Duration())
			continue
		}

		if execution.Test == (TestName{}) {
			delete(testRuns, test)
			slog.Debug("removed test run with empty test name", "test", test)
			continue
		}

		slog.Debug(
			"parsed test",
			"test", execution.Test,
			"ran_from", execution.Start,
			"to", execution.End,
			"for", execution.Duration(),
			"passed", execution.Passed,
		)

		if execution.Duration() > maxDuration {
			maxDuration = execution.Duration()
		}
	}

	slog.Debug("parsed", "start", start, "end", end)

	return ParseResult{
		TestPauses:  testPauses,
		TestRuns:    testRuns,
		Start:       start,
		End:         end,
		MaxDuration: maxDuration,
		Failed:      failed,
	}
}
