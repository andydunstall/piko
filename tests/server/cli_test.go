//go:build system

package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	pikoPath = "../../bin/piko"
)

func TestCLI(t *testing.T) {
	// Tests starting the server and configuring with flags.
	t.Run("flags", func(t *testing.T) {
		if !pikoExists() {
			t.Skip("missing piko binary")
		}

		cmd := exec.Command(
			pikoPath, "server", "--admin.bind-addr", "127.0.0.1:10000",
		)
		err := cmd.Start()
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()

		assert.NoError(t, waitForReady(ctx, "127.0.0.1:10000"))

		assert.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.NoError(t, cmd.Wait())
	})

	// Tests starting the server and configuring YAML.
	t.Run("yaml", func(t *testing.T) {
		if !pikoExists() {
			t.Skip("missing piko binary")
		}

		yaml := `
admin:
  bind_addr: 127.0.0.1:10001
`

		f, err := os.CreateTemp("", "piko")
		assert.NoError(t, err)

		_, err = f.WriteString(yaml)
		assert.NoError(t, err)

		cmd := exec.Command(
			pikoPath, "server", "--config.path", f.Name(),
		)
		err = cmd.Start()
		assert.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()

		assert.NoError(t, waitForReady(ctx, "127.0.0.1:10001"))

		assert.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
		assert.NoError(t, cmd.Wait())
	})
}

func pikoExists() bool {
	_, err := os.Open(pikoPath)
	return err == nil || !errors.Is(err, fs.ErrNotExist)
}

// waitForReady polls the server at the given address until it reports ready.
func waitForReady(ctx context.Context, addr string) error {
	client := &http.Client{}

	for {
		r, err := http.NewRequestWithContext(
			ctx, http.MethodGet, "http://"+addr+"/ready", nil,
		)
		if err != nil {
			return fmt.Errorf("request: %w", err)
		}

		resp, err := client.Do(r)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
