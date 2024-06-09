package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/andydunstall/piko/pkg/build"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/upstream"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	reportInterval = time.Hour
)

type Report struct {
	ID        string `json:"id"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Version   string `json:"version"`
	Uptime    int64  `json:"uptime"`
	Requests  uint64 `json:"requests"`
	Upstreams uint64 `json:"upstreams"`
}

// Reporter sends a periodic usage report.
type Reporter struct {
	id     string
	start  time.Time
	usage  *upstream.Usage
	logger log.Logger
}

func NewReporter(usage *upstream.Usage, logger log.Logger) *Reporter {
	return &Reporter{
		id:     uuid.New().String(),
		start:  time.Now(),
		usage:  usage,
		logger: logger.WithSubsystem("reporter"),
	}
}

func (r *Reporter) Run(ctx context.Context) {
	// Report on startup.
	r.report()

	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Report on shutdown.
			r.report()
			return
		case <-ticker.C:
			// Report on interval.
			r.report()
		}
	}
}

func (r *Reporter) report() {
	report := &Report{
		ID:        r.id,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Version:   build.Version,
		Uptime:    int64(time.Since(r.start).Seconds()),
		Requests:  r.usage.Requests.Load(),
		Upstreams: r.usage.Upstreams.Load(),
	}
	if err := r.send(report); err != nil {
		// Debug only as theres no user impact.
		r.logger.Debug("failed to send usage report", zap.Error(err))
	}
}

func (r *Reporter) send(report *Report) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, "http://report.pikoproxy.com/v1", bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
