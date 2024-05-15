package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeConfig struct {
	Foo string        `yaml:"foo"`
	Bar string        `yaml:"bar"`
	Sub fakeSubConfig `yaml:"sub"`
}

type fakeSubConfig struct {
	Car int `yaml:"car"`
}

func TestLoad(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		f, err := os.CreateTemp("", "piko")
		assert.NoError(t, err)

		_, err = f.WriteString(`foo: val1
bar: val2
sub:
  car: 5`)
		assert.NoError(t, err)

		var conf fakeConfig
		assert.NoError(t, Load(f.Name(), &conf, false))

		assert.Equal(t, "val1", conf.Foo)
		assert.Equal(t, "val2", conf.Bar)
		assert.Equal(t, 5, conf.Sub.Car)
	})

	t.Run("expand env", func(t *testing.T) {
		f, err := os.CreateTemp("", "piko")
		assert.NoError(t, err)

		_ = os.Setenv("PIKO_VAL1", "val1")
		_ = os.Setenv("PIKO_VAL2", "val2")

		_, err = f.WriteString(`foo: $PIKO_VAL1
bar: ${PIKO_VAL2}
sub:
  car: ${PIKO_VAL3:5}`)
		assert.NoError(t, err)

		var conf fakeConfig
		assert.NoError(t, Load(f.Name(), &conf, true))

		assert.Equal(t, "val1", conf.Foo)
		assert.Equal(t, "val2", conf.Bar)
		assert.Equal(t, 5, conf.Sub.Car)
	})

	t.Run("invalid yaml", func(t *testing.T) {
		f, err := os.CreateTemp("", "piko")
		assert.NoError(t, err)

		_, err = f.WriteString(`invalid yaml...`)
		assert.NoError(t, err)

		var conf fakeConfig
		assert.Error(t, Load(f.Name(), &conf, false))
	})

	t.Run("not found", func(t *testing.T) {
		var conf fakeConfig
		assert.Error(t, Load("notfound", &conf, false))
	})
}
