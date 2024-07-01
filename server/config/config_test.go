package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests the default configuration is valid (not including node ID).
func TestConfig_Default(t *testing.T) {
	conf := Default()
	conf.Cluster.NodeID = "my-node"
	assert.NoError(t, conf.Validate())
}
