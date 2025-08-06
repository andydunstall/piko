package auth

import (
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWKS_Load(t *testing.T) {
	remoteEndpoint := keyServer(t, publicJWKS1)
	localEndpoint := writeKeys(t, publicJWKS2)

	testCases := map[string]struct {
		endpoint    string
		kidsToCheck []string
		shouldFail  bool
	}{
		"remote": {
			kidsToCheck: []string{
				"_GEUy9hK_WTYI9PKwnUhEeXZw6f5uiED6mTpSJmNJ6o",
				"nf4NIGcUu45x7Fkr1euV2NGokFb6Fvsbjy6FpXtCDyw",
			},
			endpoint:   remoteEndpoint,
			shouldFail: false,
		},
		"local": {
			kidsToCheck: []string{
				"1ukSWvt7ZXTe0IMcbMkeCOz6FnKLgxfFoaB2rVscWvA",
				"Fs9zzciWmFVTI0ghEUyZJlyVSiw6yc-ydcC1Ctbth6o",
			},
			endpoint:   localEndpoint,
			shouldFail: false,
		},
		"remote, with unknown kid": {
			kidsToCheck: []string{
				"1ukSWvt7ZXTe0IMcbMkeCOz6FnKLgxfFoaB2rVscWvA",
				"Fs9zzciWmFVTI0ghEUyZJlyVSiw6yc-ydcC1Ctbth6o",
			},
			endpoint:   remoteEndpoint,
			shouldFail: true,
		},
		"local, with unknown kid": {
			kidsToCheck: []string{
				"_GEUy9hK_WTYI9PKwnUhEeXZw6f5uiED6mTpSJmNJ6o",
				"nf4NIGcUu45x7Fkr1euV2NGokFb6Fvsbjy6FpXtCDyw",
			},
			endpoint:   localEndpoint,
			shouldFail: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			jwksConfig := JWKSConfig{
				Endpoint: tc.endpoint,
				CacheTTL: time.Minute,
			}

			loadedJWKS, err := jwksConfig.Load(t.Context())
			require.NoError(t, err)

			for _, kid := range tc.kidsToCheck {
				token := &jwt.Token{
					Header: map[string]interface{}{
						"alg": "RS256",
						"kid": kid,
					},
				}

				key, err := loadedJWKS.KeyFunc(token)
				if tc.shouldFail {
					require.Error(t, err)
					continue
				}

				require.NoError(t, err)
				assert.NotEmpty(t, key)
			}
		})
	}
}

func keyServer(t *testing.T, jwks string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}

	t.Cleanup(func() {
		_ = ln.Close()
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(jwks))
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		err := http.Serve(ln, nil)
		if !errors.Is(err, net.ErrClosed) {
			require.NoError(t, err)
		}
	}()

	url := "http://" + ln.Addr().String()

	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 100*time.Millisecond)

	return url
}

func writeKeys(t *testing.T, jwks string) string {
	dir, err := os.MkdirTemp("", "piko")
	require.NoError(t, err)

	filePath := dir + "/jwks.json"
	require.NoError(t, os.WriteFile(filePath, []byte(jwks), 0644))

	return filePath
}

const (
	// Keys generated using https://mkjwk.org/.
	publicJWKS1 = `{
		"keys": [
		{
			"kty": "RSA",
			"e": "AQAB",
			"use": "sig",
			"kid": "_GEUy9hK_WTYI9PKwnUhEeXZw6f5uiED6mTpSJmNJ6o",
			"alg": "RS256",
			"n": "vOO2BDiQufa-L-xPDmkzVA5zk13wA-PD0bBIhI29DzkDDngAFJquGATD8p_mmI0g0z-qWh1bP2sg401RemyBMw8eU8bZ2owHViZigS4leTj1kWxfDf-_s934fvLoHR6kavyMefFTCYqMJbF9IXP4eZkMyu7VZMFkgLy2DiLc4zIgBXZoAkxO1wIJJSyjMht_nkVMWgp5j6JBMwGNl3d9HIhLTpCFUJjyZJ2rFVF0zl5AUN4xfTzmCKyESVXjGaitug6cHWyta-6i1xRXxLvfLfcpnx-hzdVqqSgQKOuA7WcZRtDqJbL5hSJtyHJDb7ivdY7CnaKMWo3A6TlAaSTBjw"
		},
		{
			"kty": "RSA",
			"e": "AQAB",
			"use": "sig",
			"kid": "nf4NIGcUu45x7Fkr1euV2NGokFb6Fvsbjy6FpXtCDyw",
			"alg": "RS256",
			"n": "uWfqOuT_d_NM7c5iRU_9bu-gNadfF4QRbKyZoiSrv0p0mPytjOQOjftFEIXu7iY4RE8ESXH8xmrk94B1ifSK_13j567EhgMOuW5fzrAzjrX62ao7RChkAFzUOcy7Gavwhbc5j18ixMJVLHtl3-9N0DMCo-H9nWmUM-PtmOY1oLjD04TNHEowVeyo0GSHd9xDFTMYq3cTdnNM_IJhmb02rhANPSvxtohOWzyhxq0RbAK_YO_2mwJjlT2a3R39PY7NgsXoNuZ3WnvOAKOfBEK3AcCwcw74g6i2TvxxllxtWrE5MA2Xw6Mqv0wGMxlPL1uH8B_FiZVesKd5KpqA6ENerw"
		}]
	}`

	publicJWKS2 = `{
		"keys": [
		{
			"kty": "RSA",
			"e": "AQAB",
			"use": "sig",
			"kid": "1ukSWvt7ZXTe0IMcbMkeCOz6FnKLgxfFoaB2rVscWvA",
			"alg": "RS256",
			"n": "i0NqFOFPlVtq0xuBih41PLVu9m0QL6thxeqYIt3bICR07eyGaHNkZu8PfDpP3WhVAIsWI8WeHTOsgk6qcrk9shKi6VzFFF3qS1xaEOJ5cW1kuiBDrcUNPa82No3ZGvTCjfdcKLs6LD8lg1V_r2dfoCFnRItJUEWgrzWRFWEr7fi1OOmyXySl5g41xGMVkrlDiyXbDLmUeW5EElfV8A00ABPgnFaIDx38ISvdsCx3rCRItDSxrkmfr_JvlAnAOwb3fATZvwry0dQ1MFWtNn3pYd-Gn4YhXGtfOgyClUEe4EBDItzHobrF0pmeLvgCvO6BXgTNlDfQ2fldCXI20-Ca7Q"
		},
		{
			"kty": "RSA",
			"e": "AQAB",
			"use": "sig",
			"kid": "Fs9zzciWmFVTI0ghEUyZJlyVSiw6yc-ydcC1Ctbth6o",
			"alg": "RS256",
			"n": "l8maIATYW55gNkDomOCzh4ylF7MdXb_zG4lAmGJW9oYIVGJ_2ZK7L6ugNzYPnfU0i4hWVjIumAjH_BlbyAuvfl-1um9G5zdGaxFcXj64g1VO1U5ec5sQgT4kL_WWQMPjOfzAreFMeA6m-AU5dpdqO_51wtMLD8rNFPepOfJAFCvWc_k62gIvztd1s5d21cln9mUcTFuG64u8PF1GaSJq8OMtZZ0dSshC7-GdJ-yAnMFtLcHzfeu2kMUJA3JrzWq0d0bGSCgET8ynZr7FCEzDTQqbe6tyZlLVzreYumbgsv_WXg8e-efeJRRPDuqdID5Jp527IK18cGaSRS9DMMw4Tw"
		}]
	}`
)
