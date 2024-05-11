package gossip

import (
	"sync"
	"time"
)

// arrivalIntervals tracks the intervals in a circular buffer.
type arrivalIntervals struct {
	intervals []int64
	// index points to the next entry to add an interval. Since intervals is
	// a circular buffer this wraps around.
	index  int
	isFull bool

	sum  int64
	mean float64
}

func newArrivalIntervals(sampleSize int) *arrivalIntervals {
	return &arrivalIntervals{
		intervals: make([]int64, sampleSize),
		index:     0,
		isFull:    false,
		sum:       0,
	}
}

func (i *arrivalIntervals) Mean() float64 {
	return i.mean
}

func (i *arrivalIntervals) Add(interval int64) {
	// If the index is at the end of the buffer wrap around.
	if i.index == len(i.intervals) {
		i.index = 0
		i.isFull = true
	}
	if i.isFull {
		i.sum = i.sum - i.intervals[i.index]
	}

	i.intervals[i.index] = interval
	i.index++
	i.sum += interval
	i.mean = float64(i.sum) / float64(i.size())
}

func (i *arrivalIntervals) size() int {
	if i.isFull {
		return len(i.intervals)
	}
	return i.index
}

type arrivalWindow struct {
	lastTimestamp     time.Time
	intervals         *arrivalIntervals
	bootstrapInterval time.Duration
}

func newArrivalWindow(
	bootstrapInterval time.Duration,
	sampleSize int,
) *arrivalWindow {
	return &arrivalWindow{
		intervals:         newArrivalIntervals(sampleSize),
		bootstrapInterval: bootstrapInterval,
	}
}

func (w *arrivalWindow) Phi(timestamp time.Time) float64 {
	if !(w.lastTimestamp.After(time.Time{}) && w.intervals.Mean() > 0.0) {
		panic("cannot sample phi before any samples arrived")
	}

	deltaSinceLast := timestamp.Sub(w.lastTimestamp).Nanoseconds()
	return float64(deltaSinceLast) / w.intervals.Mean()
}

func (w *arrivalWindow) Add(timestamp time.Time) {
	if w.lastTimestamp.After(time.Time{}) {
		w.intervals.Add(timestamp.Sub(w.lastTimestamp).Nanoseconds())
	} else {
		// If this is the first interval, use a high interval to avoid false
		// positives when we don't have many samples.
		w.intervals.Add(w.bootstrapInterval.Nanoseconds())
	}
	w.lastTimestamp = timestamp
}

// failureDetector monitors the liveness of known nodes based on received
// messages.
type failureDetector interface {
	Report(nodeID string)
	SuspicionLevel(nodeID string) float64
	Remove(nodeID string)
}

// accrualFailureDetector implements failureDetector using the "Phi Accrual
// Failure Detector".
type accrualFailureDetector struct {
	windows map[string]*arrivalWindow

	// mu protects the above fields
	mu sync.Mutex

	bootstrapInterval time.Duration
	sampleSize        int
}

func newAccrualFailureDetector(
	bootstrapInterval time.Duration,
	sampleSize int,
) *accrualFailureDetector {
	return &accrualFailureDetector{
		windows:           make(map[string]*arrivalWindow),
		bootstrapInterval: bootstrapInterval,
		sampleSize:        sampleSize,
	}
}

// Report reports a message was received from the node with the given ID.
func (d *accrualFailureDetector) Report(nodeID string) {
	d.ReportWithTimestamp(nodeID, time.Now())
}

// ReportWithTimestamp reports a message was received from the node with the
// given ID.
func (d *accrualFailureDetector) ReportWithTimestamp(
	nodeID string,
	timestamp time.Time,
) {
	d.mu.Lock()
	defer d.mu.Unlock()

	window, ok := d.windows[nodeID]
	if !ok {
		window = newArrivalWindow(d.bootstrapInterval, d.sampleSize)
		d.windows[nodeID] = window
	}
	window.Add(timestamp)
}

// SuspicionLevel returns the 'phi' value indicating the suspicion level of
// whether the node with the given ID is unreachable.
//
// The higher the suspicion level, the more likely the node is to be
// unreachable.
func (d *accrualFailureDetector) SuspicionLevel(nodeID string) float64 {
	return d.SuspicionLevelAt(nodeID, time.Now())
}

// SuspicionLevelAt returns the 'phi' value indicating the suspicion level of
// whether the node with the given ID is unreachable.
//
// The higher the suspicion level, the more likely the node is to be
// unreachable.
func (d *accrualFailureDetector) SuspicionLevelAt(nodeID string, timestamp time.Time) float64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	window, ok := d.windows[nodeID]
	if !ok {
		// If we have never received any heartbeats from the node, start by
		// assuming it is alive, though add an initial bootstrap interval so we
		// can eventually detect the node as unreachable if we never receive
		// any heartbeats.
		window = newArrivalWindow(d.bootstrapInterval, d.sampleSize)
		window.Add(timestamp)
		d.windows[nodeID] = window
	}

	return window.Phi(timestamp)
}

// Remove discards state on the given node.
func (d *accrualFailureDetector) Remove(nodeID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.windows, nodeID)
}

var _ failureDetector = &accrualFailureDetector{}
