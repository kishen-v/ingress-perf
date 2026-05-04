// Copyright 2023 The ingress-perf Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tools

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/cloud-bulldozer/ingress-perf/pkg/config"
)

type gowrk struct {
	cmd []string
	res PodResult
}

func init() {
	toolMap["go-wrk"] = GoWrk
}

// GoWrk creates a new go-wrk tool instance
func GoWrk(cfg config.Config, ep string) Tool {
	cmd := []string{
		"go-wrk",
		"-c", strconv.Itoa(cfg.Connections),
		"-d", fmt.Sprintf("%v", int(cfg.Duration.Seconds())),
		"-T", fmt.Sprintf("%v", int(cfg.RequestTimeout.Milliseconds())),
		"-no-vr", // Skip SSL certificate verification
	}

	// Add HTTP/2 flag if disabled (go-wrk defaults to HTTP/2 enabled)
	if !cfg.HTTP2 {
		cmd = append(cmd, "-http=false")
	}

	// Add keepalive flag if disabled (go-wrk defaults to keepalive enabled)
	if !cfg.Keepalive {
		cmd = append(cmd, "-no-ka")
	}

	// Add the endpoint URL
	cmd = append(cmd, ep)

	newGoWrk := &gowrk{
		cmd: cmd,
		res: PodResult{},
	}
	return newGoWrk
}

func (w *gowrk) Cmd() []string {
	return w.cmd
}

// ParseResult parses go-wrk output and converts it to PodResult
func (w *gowrk) ParseResult(stdout, _ string) (PodResult, error) {
	var err error

	// Parse requests per second (go-wrk outputs "Requests/sec:")
	if match := regexp.MustCompile(`Requests/sec:\s+([\d.]+)`).FindStringSubmatch(stdout); len(match) > 1 {
		w.res.AvgRps, err = strconv.ParseFloat(match[1], 64)
		if err != nil {
			return w.res, fmt.Errorf("failed to parse RPS: %w", err)
		}
	}

	// Parse average latency (go-wrk outputs "Avg Req Time:")
	// Handle both µs and ms units
	if match := regexp.MustCompile(`Avg Req Time:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return w.res, fmt.Errorf("failed to parse avg latency: %w", err)
		}
		// Convert to microseconds
		switch match[2] {
		case "µs":
			w.res.AvgLatency = latency
		case "ms":
			w.res.AvgLatency = latency * 1000
		case "s":
			w.res.AvgLatency = latency * 1000000
		}
	}

	// Parse max latency (go-wrk outputs "Slowest Request:")
	if match := regexp.MustCompile(`Slowest Request:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return w.res, fmt.Errorf("failed to parse max latency: %w", err)
		}
		// Convert to microseconds
		switch match[2] {
		case "µs":
			w.res.MaxLatency = latency
		case "ms":
			w.res.MaxLatency = latency * 1000
		case "s":
			w.res.MaxLatency = latency * 1000000
		}
	}

	// Parse 99th percentile latency (go-wrk outputs "99%:")
	if match := regexp.MustCompile(`99%:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			return w.res, fmt.Errorf("failed to parse p99 latency: %w", err)
		}
		// Convert to microseconds
		switch match[2] {
		case "µs":
			w.res.P99Latency = latency
		case "ms":
			w.res.P99Latency = latency * 1000
		case "s":
			w.res.P99Latency = latency * 1000000
		}
	}

	// Parse 95th percentile latency (go-wrk outputs "95%:" but not in standard output)
	// We'll try to parse it if available, otherwise estimate
	if match := regexp.MustCompile(`95%:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			// Convert to microseconds
			switch match[2] {
			case "µs":
				w.res.P95Latency = latency
			case "ms":
				w.res.P95Latency = latency * 1000
			case "s":
				w.res.P95Latency = latency * 1000000
			}
		}
	}

	// Parse 90th percentile latency (go-wrk outputs "90%:" but not in standard output)
	// We'll try to parse it if available, otherwise estimate
	if match := regexp.MustCompile(`90%:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			// Convert to microseconds
			switch match[2] {
			case "µs":
				w.res.P90Latency = latency
			case "ms":
				w.res.P90Latency = latency * 1000
			case "s":
				w.res.P90Latency = latency * 1000000
			}
		}
	}

	// Parse 75th percentile for better estimation if 90/95 not available
	var p75Latency float64
	if match := regexp.MustCompile(`75%:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		latency, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			// Convert to microseconds
			switch match[2] {
			case "µs":
				p75Latency = latency
			case "ms":
				p75Latency = latency * 1000
			case "s":
				p75Latency = latency * 1000000
			}
		}
	}

	// Parse total number of requests (go-wrk outputs "requests in")
	if match := regexp.MustCompile(`(\d+) requests in`).FindStringSubmatch(stdout); len(match) > 1 {
		w.res.Requests, err = strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			return w.res, fmt.Errorf("failed to parse total requests: %w", err)
		}
	}

	// Parse standard deviation (go-wrk outputs "stddev:")
	if match := regexp.MustCompile(`stddev:\s+([\d.]+)(µs|ms|s)`).FindStringSubmatch(stdout); len(match) > 2 {
		stdev, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			// Convert to microseconds
			switch match[2] {
			case "µs":
				w.res.StdevLatency = stdev
			case "ms":
				w.res.StdevLatency = stdev * 1000
			case "s":
				w.res.StdevLatency = stdev * 1000000
			}
		}
	}

	// Parse number of errors
	if match := regexp.MustCompile(`Number of Errors:\s+(\d+)`).FindStringSubmatch(stdout); len(match) > 1 {
		errors, err := strconv.ParseInt(match[1], 10, 64)
		if err == nil {
			w.res.HTTPErrors = errors
		}
	}

	// Estimate P90 and P95 if not parsed (using 75th percentile and 99th percentile)
	if w.res.P90Latency == 0 && p75Latency > 0 && w.res.P99Latency > 0 {
		// Linear interpolation: P90 ≈ P75 + 0.6 * (P99 - P75)
		w.res.P90Latency = p75Latency + 0.6*(w.res.P99Latency-p75Latency)
	}

	if w.res.P95Latency == 0 && p75Latency > 0 && w.res.P99Latency > 0 {
		// Linear interpolation: P95 ≈ P75 + 0.8 * (P99 - P75)
		w.res.P95Latency = p75Latency + 0.8*(w.res.P99Latency-p75Latency)
	}

	// If stddev wasn't parsed and we have enough data, estimate it
	if w.res.StdevLatency == 0 && w.res.P99Latency > 0 && w.res.AvgLatency > 0 {
		// Rough approximation: stdev ≈ (p99 - mean) / 2.33 for normal distribution
		w.res.StdevLatency = (w.res.P99Latency - w.res.AvgLatency) / 2.33
	}

	// Initialize status codes map (go-wrk doesn't provide detailed status code breakdown)
	w.res.StatusCodes = make(map[int]int64)
	if w.res.Requests > 0 && w.res.HTTPErrors == 0 {
		// Assume all successful if no errors reported
		w.res.StatusCodes[200] = w.res.Requests
	} else if w.res.HTTPErrors > 0 {
		// We don't know the exact breakdown, so just track successful vs errors
		w.res.StatusCodes[200] = w.res.Requests - w.res.HTTPErrors
		if w.res.HTTPErrors > 0 {
			w.res.StatusCodes[500] = w.res.HTTPErrors
		}
	}

	// Note: go-wrk doesn't provide read/write errors or timeouts separately
	// These will remain 0 as they're not available in the output

	return w.res, nil
}
