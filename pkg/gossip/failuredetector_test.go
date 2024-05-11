package gossip

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestArrivalWindow(t *testing.T) {
	tests := []struct {
		Name        string
		ExpectedPhi float64
		Timestamps  []int64
		Now         int64
		SampleSize  int
	}{
		{
			Name:        "bootstrap phi",
			ExpectedPhi: 0.05,
			Timestamps:  []int64{100},
			Now:         200,
			SampleSize:  10,
		},
		{
			Name:        "low phi",
			ExpectedPhi: 1.0,
			Timestamps:  []int64{100, 200, 300, 400, 500, 600},
			Now:         700,
			SampleSize:  5,
		},
		{
			Name:        "high phi",
			ExpectedPhi: 14.0,
			Timestamps:  []int64{100, 200, 300, 400, 500, 600},
			Now:         2000,
			SampleSize:  5,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			window := newArrivalWindow(2000, test.SampleSize)
			for _, ts := range test.Timestamps {
				window.Add(time.Unix(0, ts))
			}

			assert.InEpsilon(
				t,
				test.ExpectedPhi,
				window.Phi(time.Unix(0, test.Now)),
				0.01,
			)
		})
	}
}

func TestFailureDetector(t *testing.T) {
	tests := []struct {
		Name                   string
		ExpectedSuspicionLevel float64
		Timestamps             []int64
		Now                    int64
		SampleSize             int
	}{
		{
			Name:                   "bootstrap status",
			ExpectedSuspicionLevel: 0.05,
			Timestamps:             []int64{100},
			Now:                    200,
			SampleSize:             10,
		},
		{
			Name:                   "low phi",
			ExpectedSuspicionLevel: 1.0,
			Timestamps:             []int64{100, 200, 300, 400, 500, 600},
			Now:                    700,
			SampleSize:             5,
		},
		{
			Name:                   "high phi",
			ExpectedSuspicionLevel: 14.0,
			Timestamps:             []int64{100, 200, 300, 400, 500, 600},
			Now:                    2000,
			SampleSize:             5,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			failureDetector := newAccrualFailureDetector(2000, test.SampleSize)
			for _, ts := range test.Timestamps {
				failureDetector.ReportWithTimestamp(
					"node-1", time.Unix(0, ts),
				)
			}

			assert.InEpsilon(
				t,
				test.ExpectedSuspicionLevel,
				failureDetector.SuspicionLevelAt(
					"node-1", time.Unix(0, test.Now),
				),
				0.01,
			)
		})
	}
}
