package client

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientHTTPClient(t *testing.T) {
	t.Run("should load the configured certificate bundle into the TLS transport", func(t *testing.T) {
		certPath := writeTestCertificateBundle(t)
		client := NewClient(Context{
			Options: ContextOptions{
				Remote: RemoteOptions{
					CertPath:       certPath,
					ConnectTimeout: 250 * time.Millisecond,
					Timeout:        2 * time.Second,
				},
			},
		})

		httpClient, err := client.httpClient()

		require.NoError(t, err)
		require.NotNil(t, httpClient)
		assert.Equal(t, 2*time.Second, httpClient.Timeout)

		transport, ok := httpClient.Transport.(*http.Transport)
		require.True(t, ok)
		require.NotNil(t, transport.TLSClientConfig)
		assert.Equal(t, 250*time.Millisecond, transport.TLSHandshakeTimeout)
		require.Len(t, transport.TLSClientConfig.Certificates, 1)
		require.Len(t, transport.TLSClientConfig.Certificates[0].Certificate, 1)

		sameClient, err := client.httpClient()
		require.NoError(t, err)
		assert.Same(t, httpClient, sameClient)
	})

	t.Run("should return an error when the certificate bundle cannot be loaded", func(t *testing.T) {
		client := NewClient(Context{
			Domain:    "domain",
			URL:       "https://example.invalid",
			APIKey:    "api-key",
			Component: "component",
			Options: ContextOptions{
				Remote: RemoteOptions{
					CertPath: filepath.Join(t.TempDir(), "missing.pem"),
				},
			},
		})

		httpClient, err := client.httpClient()

		require.Error(t, err)
		assert.Nil(t, httpClient)
		assert.Contains(t, err.Error(), "loading remote certificate")
	})
}

func TestRemoteRequestsSurfaceTransportSetupErrors(t *testing.T) {
	client := NewClient(Context{
		Domain:    "domain",
		URL:       "https://example.invalid",
		APIKey:    "api-key",
		Component: "component",
		Options: ContextOptions{
			Remote: RemoteOptions{
				CertPath: filepath.Join(t.TempDir(), "missing.pem"),
			},
		},
	})

	_, err := client.GetSwitcher("MY_SWITCHER").IsOn()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "[auth] remote unavailable")
}

func writeTestCertificateBundle(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "switcher-client-go-test",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	bundlePath := filepath.Join(t.TempDir(), "client.pem")
	bundleFile, err := os.Create(bundlePath)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, bundleFile.Close())
	}()

	require.NoError(t, pem.Encode(bundleFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}))
	require.NoError(t, pem.Encode(bundleFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	return bundlePath
}
