// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTokenForIdentityAndIssuer(t *testing.T) {
	token := makeToken(t, &idToken{
		Issuer:        "https://issuer.example",
		Subject:       "octocat",
		Email:         "octo@example.com",
		EmailVerified: true,
	})

	identity, issuer, err := parseTokenForIdentityAndIssuer(token, "")
	require.NoError(t, err)
	assert.Equal(t, "octo@example.com", identity)
	assert.Equal(t, "https://issuer.example", issuer)
}

func TestParseTokenForIdentityAndIssuer_FederatedClaims(t *testing.T) {
	token := makeToken(t, &idToken{
		Issuer:          "https://issuer.example",
		Subject:         "octocat",
		EmailVerified:   false,
		FederatedClaims: &federatedClaims{ConnectorID: "https://connector.example"},
	})

	identity, issuer, err := parseTokenForIdentityAndIssuer(token, "")
	require.NoError(t, err)
	assert.Equal(t, "octocat", identity)
	assert.Equal(t, "https://connector.example", issuer)
}

func TestParseTokenForIdentityAndIssuer_FulcioConfig(t *testing.T) {
	issuerURL := "https://issuer.example"
	token := makeToken(t, &idToken{
		Issuer:  issuerURL,
		Subject: "octocat",
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fulcioConfigurationEndpoint {
			t.Fatalf("unexpected fulcio path: %s", r.URL.Path)
		}

		response := map[string]any{
			"issuers": []map[string]any{
				{
					"issuer_type":    "username",
					"subject_domain": "example.com",
				},
			},
		}

		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()

	identity, issuer, err := parseTokenForIdentityAndIssuer(token, server.URL)
	require.NoError(t, err)
	assert.Equal(t, "octocat", identity)
	assert.Equal(t, issuerURL, issuer)
}

func TestParseTokenForIdentityAndIssuer_InvalidToken(t *testing.T) {
	_, _, err := parseTokenForIdentityAndIssuer("invalid", "")
	assert.ErrorContains(t, err, "invalid token")
}

func TestParseTokenForIdentityAndIssuer_InvalidBase64(t *testing.T) {
	_, _, err := parseTokenForIdentityAndIssuer("header.@@@.sig", "")
	assert.Error(t, err)
}

func TestStringAsBoolUnmarshal(t *testing.T) {
	var value stringAsBool

	assert.NoError(t, value.UnmarshalJSON([]byte("true")))
	assert.Equal(t, stringAsBool(true), value)
	assert.Error(t, value.UnmarshalJSON([]byte("nope")))
}

func makeToken(t *testing.T, tok *idToken) string {
	t.Helper()

	payloadBytes, err := json.Marshal(tok)
	require.NoError(t, err)

	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return "header." + payload + ".sig"
}
