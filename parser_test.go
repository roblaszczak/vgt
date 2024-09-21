package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateGolden = false

func init() {
	flag.BoolVar(&updateGolden, "update-golden", false, "update golden files")
}

func TestParse(t *testing.T) {
	testOutput, err := os.ReadFile("testdata/test.json")
	require.NoError(t, err)

	scanner := bufio.NewScanner(bytes.NewBuffer(testOutput))

	parseResult := Parse(scanner)

	marshaled, err := json.MarshalIndent(parseResult, "", "  ")
	require.NoError(t, err)

	if updateGolden {
		err = os.WriteFile("testdata/golden.json", marshaled, 0644)
		require.NoError(t, err)
	} else {
		golden, err := os.ReadFile("testdata/golden.json")
		require.NoError(t, err)

		require.JSONEq(t, string(golden), string(marshaled))
	}

	charts := generateCharts(parseResult)

	html, err := render(parseResult, charts, false)
	require.NoError(t, err)

	if updateGolden {
		err = os.WriteFile("testdata/golden.html", []byte(html), 0644)
		require.NoError(t, err)
	} else {
		golden, err := os.ReadFile("testdata/golden.html")
		require.NoError(t, err)

		require.Equal(t, string(golden), html)
	}
}
