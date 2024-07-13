package traffic

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	weakrand "math/rand"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	piko "github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pikotest/workload/traffic/config"
	"github.com/andydunstall/piko/pkg/log"
)

type HTTPClient struct {
	client *http.Client

	conf *config.Config

	shutdown chan struct{}

	wg sync.WaitGroup

	logger log.Logger
}

func NewHTTPClient(conf *config.Config, logger log.Logger) *HTTPClient {
	client := &HTTPClient{
		client: &http.Client{
			Timeout: conf.Connect.Timeout,
		},
		conf:     conf,
		shutdown: make(chan struct{}),
		logger:   logger,
	}
	client.goRun()
	return client
}

func (c *HTTPClient) Close() {
	close(c.shutdown)
	c.wg.Wait()
}

func (c *HTTPClient) run() {
	ticker := time.NewTicker(time.Duration(int(time.Second) / c.conf.Rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.request()
		case <-c.shutdown:
			return
		}
	}
}

func (c *HTTPClient) request() {
	endpointID := fmt.Sprintf("endpoint-%d", weakrand.Int()%c.conf.Endpoints)

	reqBody := randomBytes(c.conf.RequestSize)

	req, _ := http.NewRequest("GET", c.conf.Connect.URL, bytes.NewReader(reqBody))
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

func (c *HTTPClient) goRun() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run()
	}()
}

type TCPClient struct {
	dialer *piko.Dialer

	conf *config.Config

	shutdown chan struct{}

	wg sync.WaitGroup

	logger log.Logger
}

func NewTCPClient(conf *config.Config, logger log.Logger) *TCPClient {
	client := &TCPClient{
		dialer:   &piko.Dialer{},
		conf:     conf,
		shutdown: make(chan struct{}),
		logger:   logger,
	}
	client.goRun()
	return client
}

func (c *TCPClient) Close() {
	close(c.shutdown)
	c.wg.Wait()
}

func (c *TCPClient) run() {
	ticker := time.NewTicker(time.Duration(int(time.Second) / c.conf.Rate))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.request()
		case <-c.shutdown:
			return
		}
	}
}

func (c *TCPClient) request() {
	endpointID := fmt.Sprintf("endpoint-%d", weakrand.Int()%c.conf.Endpoints)

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		c.conf.Connect.Timeout,
	)
	defer connectCancel()

	conn, err := c.dialer.Dial(connectCtx, endpointID)
	if err != nil {
		c.logger.Warn("dial", zap.Error(err))
		return
	}

	reqBody := randomBytes(c.conf.RequestSize)

	_, err = conn.Write(reqBody)
	if err != nil {
		c.logger.Warn("write", zap.Error(err))
		return
	}

	respBody := make([]byte, 1024)
	if _, err := io.ReadFull(conn, respBody); err != nil {
		c.logger.Warn("read", zap.Error(err))
		return
	}

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

func (c *TCPClient) goRun() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run()
	}()
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic("read rand: " + err.Error())
	}
	return b
}
