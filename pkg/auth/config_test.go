package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Load(t *testing.T) {
	t.Run("hmac", func(t *testing.T) {
		config := Config{
			HMACSecretKey: "my-secret-key",
		}
		assert.True(t, config.Enabled())

		loaded, err := config.Load()
		assert.NoError(t, err)

		assert.Equal(t, []byte("my-secret-key"), loaded.HMACSecretKey)
	})

	t.Run("rsa", func(t *testing.T) {
		config := Config{
			RSAPublicKey: `-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEA+xGZ/wcz9ugFpP07Nspo6U17l0YhFiFpxxU4pTk3Lifz9R3zsIsu
ERwta7+fWIfxOo208ett/jhskiVodSEt3QBGh4XBipyWopKwZ93HHaDVZAALi/2A
+xTBtWdEo7XGUujKDvC2/aZKukfjpOiUI8AhLAfjmlcD/UZ1QPh0mHsglRNCmpCw
mwSXA9VNmhz+PiB+Dml4WWnKW/VHo2ujTXxq7+efMU4H2fny3Se3KYOsFPFGZ1TN
QSYlFuShWrHPtiLmUdPoP6CV2mML1tk+l7DIIqXrQhLUKDACeM5roMx0kLhUWB8P
+0uj1CNlNN4JRZlC7xFfqiMbFRU9Z4N6YwIDAQAB
-----END RSA PUBLIC KEY-----
`,
		}

		assert.True(t, config.Enabled())

		loaded, err := config.Load()
		assert.NoError(t, err)

		assert.NotNil(t, loaded.RSAPublicKey)
	})

	t.Run("ecdsa", func(t *testing.T) {
		config := Config{
			ECDSAPublicKey: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEYD54V/vp+54P9DXarYqx4MPcm+HK
RIQzNasYSoRQHQ/6S6Ps8tpMcT+KvIIC8W/e9k0W7Cm72M1P9jU7SLf/vg==
-----END PUBLIC KEY-----
`,
		}

		assert.True(t, config.Enabled())

		loaded, err := config.Load()
		assert.NoError(t, err)

		assert.NotNil(t, loaded.ECDSAPublicKey)
	})
}
