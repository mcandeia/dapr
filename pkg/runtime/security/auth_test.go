package security

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"

	securityConsts "github.com/dapr/dapr/pkg/security/consts"
)

func mockGenCSR(id string) ([]byte, []byte, error) {
	return []byte{1}, []byte{2}, nil
}

func getTestAuthenticator() Authenticator {
	return newAuthenticator("test", x509.NewCertPool(), nil, nil, mockGenCSR)
}

func TestGetTrustAuthAnchors(t *testing.T) {
	a := getTestAuthenticator()
	ta := a.GetTrustAnchors()
	assert.NotNil(t, ta)
}

func TestGetCurrentSignedCert(t *testing.T) {
	a := getTestAuthenticator()
	a.(*authenticator).currentSignedCert = &SignedCertificate{}
	c := a.GetCurrentSignedCert()
	assert.NotNil(t, c)
}

func TestGetSentryIdentifier(t *testing.T) {
	t.Run("with identity in env", func(t *testing.T) {
		envID := "cluster.local"
		t.Setenv(securityConsts.SentryLocalIdentityEnvVar, envID)

		id := getSentryIdentifier("app1")
		assert.Equal(t, envID, id)
	})

	t.Run("without identity in env", func(t *testing.T) {
		id := getSentryIdentifier("app1")
		assert.Equal(t, "app1", id)
	})
}
