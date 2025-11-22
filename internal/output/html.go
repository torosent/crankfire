package output

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"time"

	"github.com/torosent/crankfire/internal/metrics"
	"github.com/torosent/crankfire/internal/threshold"
)

// HTMLReportData contains all data needed for the HTML report template.
type HTMLReportData struct {
	GeneratedAt      string
	Stats            metrics.Stats
	History          []metrics.DataPoint
	ThresholdResults []threshold.Result
	ThresholdSummary *ThresholdSummary
	HistoryJSON      string
	EndpointNames    []string
	Metadata         ReportMetadata
}

// ReportMetadata contains configuration information about the test run.
type ReportMetadata struct {
	TargetURL       string
	TestedEndpoints []TestedEndpoint
}

// TestedEndpoint represents an endpoint configuration used in the test.
type TestedEndpoint struct {
	Name   string
	Method string
	URL    string
}

// GenerateHTMLReport generates a standalone HTML report with embedded charts.
func GenerateHTMLReport(w io.Writer, stats metrics.Stats, history []metrics.DataPoint, thresholdResults []threshold.Result, metadata ReportMetadata) error {
	// Prepare threshold summary
	var thresholdSummary *ThresholdSummary
	if len(thresholdResults) > 0 {
		thresholdSummary = &ThresholdSummary{
			Total:   len(thresholdResults),
			Results: make([]ThresholdResultJSON, len(thresholdResults)),
		}
		for i, tr := range thresholdResults {
			thresholdSummary.Results[i] = ThresholdResultJSON{
				Threshold: tr.Threshold.Raw,
				Metric:    tr.Threshold.Metric,
				Aggregate: tr.Threshold.Aggregate,
				Operator:  tr.Threshold.Operator,
				Expected:  tr.Threshold.Value,
				Actual:    tr.Actual,
				Pass:      tr.Pass,
			}
			if tr.Pass {
				thresholdSummary.Passed++
			} else {
				thresholdSummary.Failed++
			}
		}
	}

	// Prepare endpoint names sorted by request count
	endpointNames := make([]string, 0, len(stats.Endpoints))
	for name := range stats.Endpoints {
		endpointNames = append(endpointNames, name)
	}
	sort.Slice(endpointNames, func(i, j int) bool {
		return stats.Endpoints[endpointNames[i]].Total > stats.Endpoints[endpointNames[j]].Total
	})

	// Convert history to JSON for embedding in HTML
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	data := HTMLReportData{
		GeneratedAt:      time.Now().Format(time.RFC3339),
		Stats:            stats,
		History:          history,
		ThresholdResults: thresholdResults,
		ThresholdSummary: thresholdSummary,
		HistoryJSON:      string(historyJSON),
		EndpointNames:    endpointNames,
		Metadata:         metadata,
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatDuration": func(d time.Duration) string {
			return d.String()
		},
		"formatFloat": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
		"formatPercent": func(part, total int64) string {
			if total == 0 {
				return "0.0"
			}
			return fmt.Sprintf("%.1f", (float64(part)/float64(total))*100)
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Crankfire Load Test Report</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #f5f7fa;
            color: #2c3e50;
            line-height: 1.6;
            padding: 20px;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px 40px;
        }
        header h1 {
            font-size: 2rem;
            margin-bottom: 10px;
        }
        header .meta {
            opacity: 0.9;
            font-size: 0.9rem;
        }
        .content {
            padding: 40px;
        }
        .grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        .card {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            border-left: 4px solid #667eea;
        }
        .card h3 {
            font-size: 0.9rem;
            color: #6c757d;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 10px;
        }
        .card .value {
            font-size: 2rem;
            font-weight: bold;
            color: #2c3e50;
        }
        .card .subvalue {
            font-size: 0.85rem;
            color: #6c757d;
            margin-top: 5px;
        }
        .card.success {
            border-left-color: #10b981;
        }
        .card.error {
            border-left-color: #ef4444;
        }
        .card.warning {
            border-left-color: #f59e0b;
        }
        .section {
            margin-bottom: 40px;
        }
        .section h2 {
            font-size: 1.5rem;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #e5e7eb;
        }
        .chart-container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            margin-bottom: 30px;
            border: 1px solid #e5e7eb;
        }
        .chart-container h3 {
            font-size: 1.1rem;
            margin-bottom: 15px;
            color: #4b5563;
        }
        .chart {
            width: 100%;
            height: 300px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: white;
        }
        th, td {
            text-align: left;
            padding: 12px;
            border-bottom: 1px solid #e5e7eb;
        }
        th {
            background: #f8f9fa;
            font-weight: 600;
            color: #4b5563;
            font-size: 0.9rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        tr:hover {
            background: #f8f9fa;
        }
        .badge {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85rem;
            font-weight: 600;
        }
        .badge-success {
            background: #d1fae5;
            color: #065f46;
        }
        .badge-error {
            background: #fee2e2;
            color: #991b1b;
        }
        .latency-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            gap: 15px;
            margin-top: 20px;
        }
        .latency-item {
            background: #f8f9fa;
            padding: 15px;
            border-radius: 6px;
            text-align: center;
        }
        .latency-item .label {
            font-size: 0.85rem;
            color: #6c757d;
            margin-bottom: 5px;
        }
        .latency-item .value {
            font-size: 1.3rem;
            font-weight: bold;
            color: #2c3e50;
        }
        .no-data {
            text-align: center;
            padding: 40px;
            color: #6c757d;
            font-style: italic;
        }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/uplot@1.6.24/dist/uPlot.iife.min.js"></script>
    <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/uplot@1.6.24/dist/uPlot.min.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>🔥 Crankfire Load Test Report</h1>
            {{if .Metadata.TargetURL}}
            <div class="meta" style="margin-top: 5px;">Target: <a href="{{.Metadata.TargetURL}}" style="color: white; text-decoration: underline;">{{.Metadata.TargetURL}}</a></div>
            {{end}}
            <div class="meta">Generated: {{.GeneratedAt}} | Duration: {{formatDuration .Stats.Duration}}</div>
        </header>
        
        <div class="content">
            <!-- Summary Cards -->
            <div class="grid">
                <div class="card">
                    <h3>Total Requests</h3>
                    <div class="value">{{.Stats.Total}}</div>
                </div>
                <div class="card success">
                    <h3>Successful</h3>
                    <div class="value">{{.Stats.Successes}}</div>
                    <div class="subvalue">{{formatPercent .Stats.Successes .Stats.Total}}%</div>
                </div>
                <div class="card error">
                    <h3>Failed</h3>
                    <div class="value">{{.Stats.Failures}}</div>
                    <div class="subvalue">{{formatPercent .Stats.Failures .Stats.Total}}%</div>
                </div>
                <div class="card">
                    <h3>Requests/sec</h3>
                    <div class="value">{{formatFloat .Stats.RequestsPerSec}}</div>
                </div>
            </div>

            <!-- Charts Section -->
            {{if .History}}
            <div class="section">
                <h2>Performance Over Time</h2>
                
                <div class="chart-container">
                    <h3>Requests Per Second</h3>
                    <div id="rps-chart" class="chart"></div>
                </div>
                
                <div class="chart-container">
                    <h3>Latency Percentiles (ms)</h3>
                    <div id="latency-chart" class="chart"></div>
                </div>
            </div>
            {{end}}

            <!-- Latency Statistics -->
            <div class="section">
                <h2>Latency Statistics</h2>
                <div class="latency-grid">
                    <div class="latency-item">
                        <div class="label">Min</div>
                        <div class="value">{{formatDuration .Stats.MinLatency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">Max</div>
                        <div class="value">{{formatDuration .Stats.MaxLatency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">Mean</div>
                        <div class="value">{{formatDuration .Stats.MeanLatency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">P50</div>
                        <div class="value">{{formatDuration .Stats.P50Latency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">P90</div>
                        <div class="value">{{formatDuration .Stats.P90Latency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">P95</div>
                        <div class="value">{{formatDuration .Stats.P95Latency}}</div>
                    </div>
                    <div class="latency-item">
                        <div class="label">P99</div>
                        <div class="value">{{formatDuration .Stats.P99Latency}}</div>
                    </div>
                </div>
            </div>

            <!-- Thresholds -->
            {{if .ThresholdSummary}}
            <div class="section">
                <h2>Thresholds ({{.ThresholdSummary.Passed}}/{{.ThresholdSummary.Total}} Passed)</h2>
                <table>
                    <thead>
                        <tr>
                            <th>Threshold</th>
                            <th>Metric</th>
                            <th>Expected</th>
                            <th>Actual</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .ThresholdSummary.Results}}
                        <tr>
                            <td>{{.Threshold}}</td>
                            <td>{{.Metric}} ({{.Aggregate}})</td>
                            <td>{{.Operator}} {{formatFloat .Expected}}</td>
                            <td>{{formatFloat .Actual}}</td>
                            <td>
                                {{if .Pass}}
                                <span class="badge badge-success">✓ PASS</span>
                                {{else}}
                                <span class="badge badge-error">✗ FAIL</span>
                                {{end}}
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}

            <!-- Endpoint Breakdown -->
            {{if .EndpointNames}}
            <div class="section">
                <h2>Endpoint Breakdown</h2>
                <table>
                    <thead>
                        <tr>
                            <th>Endpoint</th>
                            <th>Total</th>
                            <th>Success</th>
                            <th>Failed</th>
                            <th>RPS</th>
                            <th>P99 Latency</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .EndpointNames}}
                        {{$ep := index $.Stats.Endpoints .}}
                        <tr>
                            <td><strong>{{.}}</strong></td>
                            <td>{{$ep.Total}} ({{formatPercent $ep.Total $.Stats.Total}}%)</td>
                            <td>{{$ep.Successes}}</td>
                            <td>{{$ep.Failures}}</td>
                            <td>{{formatFloat $ep.RequestsPerSec}}</td>
                            <td>{{formatDuration $ep.P99Latency}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}

            <!-- Configuration Details -->
            {{if .Metadata.TestedEndpoints}}
            <div class="section">
                <h2>Tested Endpoints Configuration</h2>
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Method</th>
                            <th>URL/Path</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Metadata.TestedEndpoints}}
                        <tr>
                            <td>{{if .Name}}<strong>{{.Name}}</strong>{{else}}<em>(default)</em>{{end}}</td>
                            <td>{{if .Method}}<span class="badge">{{.Method}}</span>{{else}}-{{end}}</td>
                            <td>{{.URL}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}
        </div>
    </div>

    {{if .History}}
    <script>
        // Prepare data for charts
        const historyJSON = {{.HistoryJSON}};
        const history = JSON.parse(historyJSON);
        
        if (history && history.length > 0) {
            // Extract timestamps and convert to seconds from start
            const startTime = new Date(history[0].timestamp).getTime();
            const timestamps = history.map(d => (new Date(d.timestamp).getTime() - startTime) / 1000);
            
            // RPS Chart
            const rpsData = [
                timestamps,
                history.map(d => d.current_rps)
            ];
            
            new uPlot({
                title: "Requests Per Second",
                width: document.getElementById('rps-chart').offsetWidth,
                height: 300,
                scales: { x: { time: false } },
                series: [
                    { label: "Time (s)" },
                    { 
                        label: "RPS",
                        stroke: "#667eea",
                        fill: "rgba(102, 126, 234, 0.1)",
                        width: 2
                    }
                ],
                axes: [
                    { label: "Time (seconds)" },
                    { label: "Requests/sec" }
                ]
            }, rpsData, document.getElementById('rps-chart'));
            
            // Latency Chart
            const latencyData = [
                timestamps,
                history.map(d => d.p50_latency_ms),
                history.map(d => d.p95_latency_ms),
                history.map(d => d.p99_latency_ms)
            ];
            
            new uPlot({
                title: "Latency Percentiles",
                width: document.getElementById('latency-chart').offsetWidth,
                height: 300,
                scales: { x: { time: false } },
                series: [
                    { label: "Time (s)" },
                    { 
                        label: "P50",
                        stroke: "#10b981",
                        width: 2
                    },
                    { 
                        label: "P95",
                        stroke: "#f59e0b",
                        width: 2
                    },
                    { 
                        label: "P99",
                        stroke: "#ef4444",
                        width: 2
                    }
                ],
                axes: [
                    { label: "Time (seconds)" },
                    { label: "Latency (ms)" }
                ]
            }, latencyData, document.getElementById('latency-chart'));
        }
    </script>
    {{end}}
</body>
</html>
`
