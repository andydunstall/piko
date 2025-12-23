package bench

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	weakrand "math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/dragonflydb/piko/bench/config"
	piko "github.com/dragonflydb/piko/client"
	"github.com/dragonflydb/piko/pkg/log"
)

type HTTPBenchmark struct {
	conf *config.Config

	wg sync.WaitGroup

	logger log.Logger
}

func NewHTTPBenchmark(conf *config.Config, logger log.Logger) *HTTPBenchmark {
	return &HTTPBenchmark{
		conf:   conf,
		logger: logger,
	}
}

func (b *HTTPBenchmark) Run(_ context.Context) error {
	nextEndpointID := 0
	for i := 0; i != b.conf.Upstreams; i++ {
		endpointID := fmt.Sprintf("endpoint-%d", nextEndpointID)
		upstream, err := newHTTPUpstream(endpointID, b.conf, b.logger)
		if err != nil {
			return fmt.Errorf("http upstream: %w", err)
		}

		defer upstream.Close()

		b.wg.Add(1)
		go func() {
			defer b.wg.Done()

			b.logger.Debug(
				"starting upstream",
				zap.String("endpoint-id", endpointID),
			)

			upstream.Start()
		}()

		nextEndpointID++
		nextEndpointID %= b.conf.Endpoints
	}

	s := time.Now()

	for _, r := range b.requestsPerClient() {
		client := newHTTPClient(b.conf, b.logger)

		b.wg.Add(1)
		go func() {
			defer b.wg.Done()

			b.logger.Debug("starting client", zap.Int("requests", r))

			client.Run(r)
		}()
	}

	b.wg.Wait()

	fmt.Printf(
		"Throughput: %.1f requests/sec\n",
		float64(b.conf.Requests)/time.Since(s).Seconds(),
	)

	return nil
}

// requestsPerClient distributes the requests among the clients.
func (b *HTTPBenchmark) requestsPerClient() []int {
	counts := make([]int, b.conf.Clients)
	perClient := b.conf.Requests / b.conf.Clients
	for i := 0; i < b.conf.Clients; i++ {
		counts[i] = perClient
	}
	extra := b.conf.Requests % b.conf.Clients
	for i := 0; i < extra; i++ {
		counts[i]++
	}
	return counts
}

type httpUpstream struct {
	server *httptest.Server
}

func newHTTPUpstream(endpointID string, conf *config.Config, logger log.Logger) (*httpUpstream, error) {
	url, _ := url.Parse(conf.Connect.UpstreamURL)
	upstream := piko.Upstream{
		URL:    url,
		Logger: logger.WithSubsystem("client"),
	}

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		conf.Connect.Timeout,
	)
	defer connectCancel()

	ln, err := upstream.Listen(connectCtx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	// HTTP server to echo the incoming request body.
	server := httptest.NewUnstartedServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Note can't use io.Copy as not supported by http.ResponseWriter.
			b, err := io.ReadAll(r.Body)
			if err != nil {
				panic(fmt.Sprintf("read body: %s", err.Error()))
			}
			n, err := w.Write(b)
			if err != nil {
				panic(fmt.Sprintf("write bytes: %d: %s", n, err))
			}
		},
	))
	server.Listener = ln

	return &httpUpstream{
		server: server,
	}, nil
}

func (u *httpUpstream) Start() {
	u.server.Start()
}

func (u *httpUpstream) Close() {
	u.server.Close()
}

type httpClient struct {
	client *http.Client

	conf *config.Config

	logger log.Logger
}

func newHTTPClient(conf *config.Config, logger log.Logger) *httpClient {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		// Override MaxIdleConnsPerHost to avoid opening too many connections
		// to the target server.
		MaxIdleConnsPerHost: 100,
	}
	client := &httpClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   conf.Connect.Timeout,
		},
		conf:   conf,
		logger: logger,
	}
	return client
}

func (c *httpClient) Run(requests int) {
	for i := 0; i != requests; i++ {
		c.request()
	}
}

func (c *httpClient) request() {
	endpointID := fmt.Sprintf("endpoint-%d", weakrand.Int()%c.conf.Endpoints)

	reqBody := randomBytes(c.conf.Size)

	req, _ := http.NewRequest("GET", c.conf.Connect.ProxyURL, bytes.NewReader(reqBody))
	req.Header.Set("x-piko-endpoint", endpointID)

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Warn("request", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("bad status", zap.Int("status", resp.StatusCode))
		return
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		c.logger.Warn("read", zap.Error(err))
		return
	}
	respBody := buf.Bytes()

	if len(reqBody) != len(respBody) {
		c.logger.Error("unexpected response body")
		return
	}
	for i := 0; i != len(reqBody); i++ {
		if reqBody[i] != respBody[i] {
			c.logger.Error("unexpected response body")
			return
		}
	}
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic("read rand: " + err.Error())
	}
	return b
}
