package setreport

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/torosent/crankfire/internal/setrunner"
	"github.com/torosent/crankfire/internal/store"
)

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Set Run — {{.Run.SetName}}</title>
<style>
body{font-family:-apple-system,Segoe UI,sans-serif;margin:2rem;color:#222;}
h1{margin-bottom:.2rem;}
.meta{color:#666;margin-bottom:1.5rem;}
.badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:.85rem;font-weight:600;}
.badge.completed{background:#d4edda;color:#155724;}
.badge.failed{background:#f8d7da;color:#721c24;}
.badge.cancelled{background:#fff3cd;color:#856404;}
.stage{margin:1.5rem 0;border:1px solid #e0e0e0;border-radius:6px;padding:1rem;}
.item{padding:.5rem;border-bottom:1px solid #f0f0f0;}
.item:last-child{border-bottom:none;}
table{border-collapse:collapse;width:100%;margin-top:1rem;}
th,td{padding:.5rem .75rem;border:1px solid #e0e0e0;text-align:right;}
th:first-child,td:first-child{text-align:left;}
th{background:#f7f7f7;}
.winner{background:#d4edda;font-weight:600;}
.threshold{padding:.5rem;}
.threshold.passed{color:#155724;}
.threshold.failed{color:#721c24;font-weight:600;}
</style>
</head>
<body>
<h1>{{.Run.SetName}} <span class="badge {{.Run.Status}}">{{.Run.Status}}</span></h1>
<div class="meta">
  Started {{.Run.StartedAt.Format "2006-01-02 15:04:05 MST"}} · Duration {{.Duration}}
</div>

<h2>Thresholds</h2>
{{if .Run.Thresholds}}
<ul>
  {{range .Run.Thresholds}}
  <li class="threshold {{if .Passed}}passed{{else}}failed{{end}}">
    {{.Scope}}: {{.Metric}} {{.Op}} {{.Value}} → actual {{printf "%.3f" .Actual}} {{if .Passed}}✓{{else}}✗{{end}}
  </li>
  {{end}}
</ul>
{{else}}
<p><em>No thresholds defined.</em></p>
{{end}}

<h2>Stages</h2>
{{range .Run.Stages}}
<div class="stage">
  <h3>{{.Name}}</h3>
  {{range .Items}}
  <div class="item">
    <strong>{{.Name}}</strong>
    <span class="badge {{.Status}}">{{.Status}}</span>
    — p50 {{printf "%.0fms" .Summary.P50Ms}},
      p95 {{printf "%.0fms" .Summary.P95Ms}},
      p99 {{printf "%.0fms" .Summary.P99Ms}},
      RPS {{printf "%.1f" (rps .Summary)}},
      err {{printf "%.2f%%" (errRate .Summary)}}
    {{if .Error}}<div style="color:#721c24">Error: {{.Error}}</div>{{end}}
  </div>
  {{end}}
</div>
{{end}}

<h2>Compare</h2>
<table class="compare-table">
  <thead><tr><th>Metric</th>{{range .Compare.Items}}<th>{{.}}</th>{{end}}</tr></thead>
  <tbody>
  {{range .Compare.Rows}}
  <tr>
    <td>{{.Metric}}</td>
    {{$row := .}}
    {{range $.Compare.Items}}
      {{$v := index $row.Values .}}
      <td class="{{if eq $row.Winner .}}winner winner-{{.}}{{end}}">{{printf "%.2f" $v}}</td>
    {{end}}
  </tr>
  {{end}}
  </tbody>
</table>
</body>
</html>
`

func rps(s store.RunSummary) float64 {
	if s.DurationSec > 0 {
		return float64(s.TotalRequests) / s.DurationSec
	}
	return 0
}

func errRate(s store.RunSummary) float64 {
	if s.TotalRequests > 0 {
		return float64(s.Errors) / float64(s.TotalRequests)
	}
	return 0
}

type renderData struct {
	Run      store.SetRun
	Compare  setrunner.Comparison
	Duration string
}

func Render(run store.SetRun) ([]byte, error) {
	var items []store.ItemResult
	for _, st := range run.Stages {
		items = append(items, st.Items...)
	}
	cmp := setrunner.Compare(items)

	dur := "—"
	if !run.EndedAt.IsZero() && !run.StartedAt.IsZero() {
		dur = run.EndedAt.Sub(run.StartedAt).Round(time.Millisecond).String()
	}

	tmpl, err := template.New("setrun").Funcs(template.FuncMap{
		"rps":     rps,
		"errRate": errRate,
	}).Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, renderData{Run: run, Compare: cmp, Duration: dur}); err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	return buf.Bytes(), nil
}
