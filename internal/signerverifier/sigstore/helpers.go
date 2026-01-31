// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	fulciogrpc "github.com/sigstore/fulcio/pkg/generated/protobuf"
)

const fulcioConfigurationEndpoint = "/api/v2/configuration"

func parseTokenForIdentityAndIssuer(token, fulcioURL string) (string, string, error) {
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) < 3 {
		return "", "", fmt.Errorf("invalid token")
	}

	token = tokenParts[1]
	tokenBytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", "", err
	}

	tok := &idToken{}
	if err := json.Unmarshal(tokenBytes, tok); err != nil {
		return "", "", err
	}

	issuer := issuerFromToken(tok)
	identity := subjectFromToken(tok)

	if fulcioURL != "" {
		slog.Debug(fmt.Sprintf("Querying '%s' for IDP configurations to see if a subject domain applies...", fulcioURL))

		fulcio, err := url.Parse(fulcioURL)
		if err != nil {
			return "", "", fmt.Errorf("unable to query Fulcio instance '%s': %w", fulcioURL, err)
		}

		fulcio.Path = fulcioConfigurationEndpoint
		configurationEndpoint := fulcio.String()

		response, err := http.Get(configurationEndpoint) //nolint:gosec
		if err != nil {
			return "", "", fmt.Errorf("unable to query Fulcio instance '%s': %w", fulcioURL, err)
		}

		responseData, err := io.ReadAll(response.Body)
		if err != nil {
			return "", "", fmt.Errorf("unable to query Fulcio instance '%s': %w", fulcioURL, err)
		}

		type configResponse struct {
			Issuers []*fulciogrpc.OIDCIssuer `json:"issuers"`
		}

		responseObj := configResponse{Issuers: []*fulciogrpc.OIDCIssuer{}}
		if err := json.Unmarshal(responseData, &responseObj); err != nil {
			return "", "", fmt.Errorf("unable to query Fulcio instance '%s': %w", fulcioURL, err)
		}

		for _, issuerConfig := range responseObj.Issuers {
			if issuerConfig.GetIssuerUrl() != issuer {
				continue
			}

			issuerType := issuerConfig.GetIssuerType()

			if issuerType == "" {
				slog.Debug("Fulcio instance does not list issuer type, cannot determine if subject domain must be added to identity")
				break
			}

			if issuerType == "username" || issuerType == "uri" {
				subjectDomain := issuerConfig.GetSubjectDomain()
				if subjectDomain == "" {
					slog.Debug("Fulcio instance lists issuer type but does not list subject domain, cannot determine subject domain to add to identity")
					break
				}

				// Per the Fulcio spec, the subject domain is added after a '!'
				slog.Debug(fmt.Sprintf("Adding subject domain '%s' to identity '%s'...", subjectDomain, identity))
				identity = fmt.Sprintf("%s!%s", identity, subjectDomain)
			}

			break
		}
	}

	return identity, issuer, nil
}

type idToken struct {
	Issuer          string           `json:"iss"`
	Subject         string           `json:"sub"`
	Email           string           `json:"email"`
	EmailVerified   stringAsBool     `json:"email_verified"`
	FederatedClaims *federatedClaims `json:"federated_claims"`
}

type stringAsBool bool

func (sb *stringAsBool) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true", `"true"`, "True", `"True"`:
		*sb = true
	case "false", `"false"`, "False", `"False"`:
		*sb = false
	default:
		return errors.New("invalid value for boolean")
	}
	return nil
}

type federatedClaims struct {
	ConnectorID string `json:"connector_id"`
}

func issuerFromToken(tok *idToken) string {
	if tok.FederatedClaims != nil && tok.FederatedClaims.ConnectorID != "" {
		return tok.FederatedClaims.ConnectorID
	}

	return tok.Issuer
}

func subjectFromToken(tok *idToken) string {
	if tok.Email != "" && tok.EmailVerified {
		return tok.Email
	}

	return tok.Subject
}
