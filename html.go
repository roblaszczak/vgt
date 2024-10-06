package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"
)

type PlotlyChart struct {
	Type         string    `json:"type"`
	Y            []string  `json:"y"`
	X            []float64 `json:"x"`
	Orientation  string    `json:"orientation"`
	Base         []float64 `json:"base"`
	Text         []string  `json:"text"`
	Textposition string    `json:"textposition"`
	Width        []float64 `json:"width"`
	Marker       struct {
		Color []string `json:"color"`
	} `json:"marker"`
	Hoverinfo string `json:"hoverinfo"`
}

func (c *PlotlyChart) Add(
	label, y string,
	start, duration time.Duration,
	color string,
) {
	c.Y = append(c.Y, y)

	c.X = append(c.X, duration.Round(time.Millisecond*10).Seconds())
	c.Base = append(c.Base, start.Round(time.Millisecond*10).Seconds())
	c.Text = append(c.Text, label)
	c.Width = append(c.Width, 0.9)
	c.Marker.Color = append(c.Marker.Color, color)
}

func render(pr ParseResult, charts []PlotlyChart, callOnLoad bool) (string, error) {
	settings := map[string]any{
		"showlegend": false,
		"yaxis": map[string]any{
			"visible": false,
		},
		"xaxis": map[string]any{
			"ticksuffix": "s",
		},
	}

	slices.Reverse(charts)

	chartsJSON, err := json.MarshalIndent(charts, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling charts: %w", err)
	}

	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling settings: %w", err)
	}

	slog.Debug("Generated HTML with charts", "charts", string(chartsJSON))

	html := `
<!DOCTYPE html>
<meta charset="utf-8">
<html>
<head>
	<title>Test Results ({{.duration}} {{.passed}} passed, {{.failed}} failed)</title>
</head>
<body>
	<div id="popover" class="popover">
        <div class="arrow-top-right"></div>
        <div class="arrow-bottom-left"></div>
		<span class="close-btn" onclick="closePopover()">&times;</span>
        <p>You can zoom chart with controls or by clicking and selecting area to zoom.</p>
    </div>
	<div id="chart"></div>
</body>

<style>
body {
	-webkit-font-smoothing: antialiased;
}

#chart {
    width: 100%;
    min-height: 100vh;
    max-height: 100%;
    position: absolute;
    top: 0;
    left: 0;
    margin: 0 auto;
}

.popover {
    font-family: "Open Sans", verdana, arial, sans-serif;
    position: fixed;
    top: 60px;
    right: 60px;
    transform: none;
    background-color: #f0f0f0;
    border: 1px solid #999;
    padding: 20px;
    box-shadow: 0 0 10px rgba(0,0,0,0.2);
    display: none;
    z-index: 1000;
	width: 330px;
}
.arrow-top-right {
    position: absolute;
    top: -10px;
    right: 10px;
    width: 0;
    height: 0;
    border-left: 10px solid transparent;
    border-right: 10px solid transparent;
    border-bottom: 10px solid #999;
}
.arrow-bottom-left {
    position: absolute;
    bottom: -10px;
    left: 10px;
    width: 0;
    height: 0;
    border-left: 10px solid transparent;
    border-right: 10px solid transparent;
    border-top: 10px solid #999;
}
.close-btn {
	cursor: pointer;
	float: right;
</style>

<script>
	{{ .plotly }}
</script>

<script>
	CHART = document.getElementById('chart');
	Plotly.newPlot( 
		CHART, 
		{{ .chartsJSON }}, 
		{{ .settingsJSON }}
    );

</script>

{{ if .callOnLoad }}
<script>
	// Send a request to /loaded when the page finishes loading
	window.onload = function() {
		fetch('/loaded').then(() => {
			console.log('Loaded signal sent');
		});
	}
</script>
{{ end }}
 <script>
    function setCookie(name, value, days) {
        var expires = "";
        if (days) {
            var date = new Date();
            date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
            expires = "; expires=" + date.toUTCString();
        }
        document.cookie = name + "=" + (value || "")  + expires + "; path=/";
    }

    function getCookie(name) {
        var nameEQ = name + "=";
        var ca = document.cookie.split(';');
        for(var i=0; i < ca.length; i++) {
            var c = ca[i];
            while (c.charAt(0) == ' ') c = c.substring(1, c.length);
            if (c.indexOf(nameEQ) == 0) return c.substring(nameEQ.length, c.length);
        }
        return null;
    }

    document.addEventListener('DOMContentLoaded', function() {
        if (!getCookie('popoverShown')) {
            document.getElementById('popover').style.display = 'block';
        }
    });

    function closePopover() {
        document.getElementById('popover').style.display = 'none';
        setCookie('popoverShown', 'true', 30); // Cookie will expire after 30 days
    }
</script>
</html>
`
	t, err := template.New("template").Parse(html)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	t = t.Option("missingkey=error")

	passed := 0
	failed := 0

	for _, execution := range pr.TestRuns {
		if execution.Passed {
			passed++
		} else {
			failed++
		}
	}

	duration := pr.End.Sub(pr.Start)

	buf := new(strings.Builder)
	err = t.Execute(buf, map[string]any{
		"plotly":       template.JS(plotly),
		"chartsJSON":   template.JS(chartsJSON),
		"settingsJSON": template.JS(settingsJSON),
		"callOnLoad":   callOnLoad,
		"passed":       passed,
		"failed":       failed,
		"duration":     duration.Round(time.Millisecond).String(),
	})
	if err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

func floatToColor(value float64) string {
	value = math.Max(0, math.Min(1, value))

	r := uint8(math.Round(60 * value)) // Reduced red component
	g := uint8(math.Round(180 * (1 - value)))
	b := uint8(math.Round(200 + 30*value))

	return fmt.Sprintf("rgba(%d, %d, %d, 100)", r, g, b)
}

func durationToRgb(d TestExecution, maxDuration time.Duration) string {
	if !d.Passed {
		return "rgba(255, 0, 0, 100)"
	}

	position := float64(d.Duration()) / float64(maxDuration)

	slog.Debug("Duration to RGB", "duration", d, "maxDuration", maxDuration, "position", position)

	return floatToColor(position)
}
