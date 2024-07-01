package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests the default configuration is valid.
func TestConfig_Default(t *testing.T) {
	conf := Default()
	assert.NoError(t, conf.Validate())
}
