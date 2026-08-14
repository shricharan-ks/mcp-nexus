/*
Copyright 2026 The MCP Gateway Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package keycloak provides a Go client for the Keycloak Admin REST API.
// It supports creating and managing OAuth2 clients, retrieving admin tokens,
// and other operations needed by the MCP Gateway for identity management.
package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// --------------------------------------------------------------------------
// Types
// --------------------------------------------------------------------------

// Client is a Keycloak Admin REST API client.
type Client struct {
	// BaseURL is the root URL of the Keycloak server (e.g. http://localhost:8080).
	BaseURL string

	// Realm is the Keycloak realm to manage.
	Realm string

	// HTTPClient is the underlying HTTP client used for requests.
	HTTPClient *http.Client
}

// NewClient creates a new Keycloak admin client.
func NewClient(baseURL, realm string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Realm:      realm,
		HTTPClient: &http.Client{},
	}
}

// ClientRepresentation mirrors the Keycloak client representation used by
// the Admin REST API.
type ClientRepresentation struct {
	// ID is the internal UUID assigned by Keycloak (read-only on create).
	ID string `json:"id,omitempty"`

	// ClientID is the client identifier (e.g. "mcp-gateway-proxy").
	ClientID string `json:"clientId"`

	// Enabled indicates whether the client is active.
	Enabled bool `json:"enabled"`

	// Secret is the client secret (only returned when fetched explicitly).
	Secret string `json:"secret,omitempty"`

	// ServiceAccountsEnabled enables the client_credentials grant.
	ServiceAccountsEnabled bool `json:"serviceAccountsEnabled"`

	// PublicClient marks the client as public (no secret). Should be false
	// for confidential clients.
	PublicClient bool `json:"publicClient"`

	// StandardFlowEnabled enables the authorization code flow.
	StandardFlowEnabled bool `json:"standardFlowEnabled"`

	// ClientAuthenticatorType is the authenticator type (e.g. "client-secret").
	ClientAuthenticatorType string `json:"clientAuthenticatorType,omitempty"`

	// ProtocolMappers defines protocol mappers attached to this client.
	ProtocolMappers []ProtocolMapper `json:"protocolMappers,omitempty"`
}

// ProtocolMapper represents a Keycloak protocol mapper configuration.
type ProtocolMapper struct {
	// Name is the display name of the mapper.
	Name string `json:"name"`

	// Protocol is the protocol this mapper applies to (e.g. "openid-connect").
	Protocol string `json:"protocol"`

	// ProtocolMapper is the mapper type identifier
	// (e.g. "oidc-hardcoded-claim-mapper").
	ProtocolMapper string `json:"protocolMapper"`

	// Config holds mapper-specific configuration key-value pairs.
	Config map[string]string `json:"config,omitempty"`
}

// tokenResponse is the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// clientSecretResponse is the response from the client-secret endpoint.
type clientSecretResponse struct {
	Value string `json:"value"`
}

// --------------------------------------------------------------------------
// Public methods
// --------------------------------------------------------------------------

// GetAdminToken obtains an admin access token from the master realm using
// the resource-owner password grant.
func (c *Client) GetAdminToken(ctx context.Context, username, password string) (string, error) {
	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", c.BaseURL)

	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {username},
		"password":   {password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	return tok.AccessToken, nil
}

// CreateClient creates an OAuth2 client in the configured realm. If the
// client already exists (HTTP 409), the conflict is handled gracefully by
// fetching and returning the existing client's secret. On success it returns
// the client secret.
func (c *Client) CreateClient(ctx context.Context, adminToken string, rep ClientRepresentation) (string, error) {
	createURL := fmt.Sprintf("%s/admin/realms/%s/clients", c.BaseURL, c.Realm)

	payload, err := json.Marshal(rep)
	if err != nil {
		return "", fmt.Errorf("marshalling client representation: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, strings.NewReader(string(payload)))
	if err != nil {
		return "", fmt.Errorf("creating client request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending create client request: %w", err)
	}
	defer resp.Body.Close()

	// Drain the body so the connection can be reused.
	_, _ = io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusCreated:
		// Client created successfully -- fetch its UUID and retrieve the secret.
		uuid, err := c.findClientUUID(ctx, adminToken, rep.ClientID)
		if err != nil {
			return "", fmt.Errorf("finding newly created client UUID: %w", err)
		}
		return c.GetClientSecret(ctx, adminToken, uuid)

	case http.StatusConflict:
		// Client already exists -- fetch existing secret.
		uuid, err := c.findClientUUID(ctx, adminToken, rep.ClientID)
		if err != nil {
			return "", fmt.Errorf("finding existing client UUID: %w", err)
		}
		return c.GetClientSecret(ctx, adminToken, uuid)

	default:
		return "", fmt.Errorf("create client failed with status %d", resp.StatusCode)
	}
}

// GetClientSecret retrieves the secret for a client identified by its
// internal UUID.
func (c *Client) GetClientSecret(ctx context.Context, adminToken, clientUUID string) (string, error) {
	secretURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s/client-secret", c.BaseURL, c.Realm, clientUUID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, secretURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating secret request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending secret request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading secret response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get client secret failed with status %d: %s", resp.StatusCode, string(body))
	}

	var secret clientSecretResponse
	if err := json.Unmarshal(body, &secret); err != nil {
		return "", fmt.Errorf("decoding secret response: %w", err)
	}

	return secret.Value, nil
}

// DeleteClient deletes a client by its clientId (not UUID). It first looks
// up the internal UUID and then issues the delete. If the client does not
// exist, no error is returned.
func (c *Client) DeleteClient(ctx context.Context, adminToken, clientID string) error {
	uuid, err := c.findClientUUID(ctx, adminToken, clientID)
	if err != nil {
		return err
	}
	if uuid == "" {
		// Client does not exist -- nothing to delete.
		return nil
	}

	deleteURL := fmt.Sprintf("%s/admin/realms/%s/clients/%s", c.BaseURL, c.Realm, uuid)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("creating delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending delete request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete client failed with status %d", resp.StatusCode)
	}

	return nil
}

// --------------------------------------------------------------------------
// Private helpers
// --------------------------------------------------------------------------

// findClientUUID looks up a client's internal UUID by its clientId string.
// Returns an empty string (without error) if the client is not found.
func (c *Client) findClientUUID(ctx context.Context, adminToken, clientID string) (string, error) {
	searchURL := fmt.Sprintf("%s/admin/realms/%s/clients?clientId=%s",
		c.BaseURL, c.Realm, url.QueryEscape(clientID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending search request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading search response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search clients failed with status %d: %s", resp.StatusCode, string(body))
	}

	var clients []ClientRepresentation
	if err := json.Unmarshal(body, &clients); err != nil {
		return "", fmt.Errorf("decoding search response: %w", err)
	}

	for _, cl := range clients {
		if cl.ClientID == clientID {
			return cl.ID, nil
		}
	}

	return "", nil
}
