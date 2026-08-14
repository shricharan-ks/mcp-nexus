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

package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --------------------------------------------------------------------------
// Test helpers
// --------------------------------------------------------------------------

// fakeClientUUID is the UUID returned by the mock Keycloak server for newly
// created or looked-up clients.
const fakeClientUUID = "f47ac10b-58cc-4372-a567-0e02b2c3d479"

// fakeClientSecret is the secret returned by the mock secret endpoint.
const fakeClientSecret = "super-secret-value"

// fakeAdminToken is the access token returned by the mock token endpoint.
const fakeAdminToken = "fake-admin-token-12345"

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestGetAdminToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request targets the master realm token endpoint.
		assert.Equal(t, "/realms/master/protocol/openid-connect/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		require.NoError(t, r.ParseForm())
		assert.Equal(t, "password", r.FormValue("grant_type"))
		assert.Equal(t, "admin-cli", r.FormValue("client_id"))
		assert.Equal(t, "admin", r.FormValue("username"))
		assert.Equal(t, "admin", r.FormValue("password"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(tokenResponse{
			AccessToken: fakeAdminToken,
			TokenType:   "Bearer",
			ExpiresIn:   300,
		})
	}))
	defer srv.Close()

	kc := NewClient(srv.URL, "mcp-gateway")
	token, err := kc.GetAdminToken(context.Background(), "admin", "admin")

	require.NoError(t, err)
	assert.Equal(t, fakeAdminToken, token)
}

func TestCreateClient_Success(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch {
		// 1st call: POST to create the client.
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/mcp-gateway/clients":
			var rep ClientRepresentation
			require.NoError(t, json.NewDecoder(r.Body).Decode(&rep))
			assert.Equal(t, "my-client", rep.ClientID)
			assert.True(t, rep.Enabled)

			w.WriteHeader(http.StatusCreated)

		// 2nd call: GET to find the client UUID.
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/mcp-gateway/clients":
			assert.Equal(t, "my-client", r.URL.Query().Get("clientId"))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ClientRepresentation{
				{ID: fakeClientUUID, ClientID: "my-client"},
			})

		// 3rd call: GET to fetch the client secret.
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/mcp-gateway/clients/"+fakeClientUUID+"/client-secret":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(clientSecretResponse{Value: fakeClientSecret})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	kc := NewClient(srv.URL, "mcp-gateway")
	secret, err := kc.CreateClient(context.Background(), fakeAdminToken, ClientRepresentation{
		ClientID:               "my-client",
		Enabled:                true,
		ServiceAccountsEnabled: true,
		PublicClient:           false,
	})

	require.NoError(t, err)
	assert.Equal(t, fakeClientSecret, secret)
	assert.Equal(t, 3, callCount, "expected exactly 3 HTTP calls (create, find, secret)")
}

func TestCreateClient_AlreadyExists(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch {
		// 1st call: POST returns 409 Conflict (client already exists).
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/mcp-gateway/clients":
			w.WriteHeader(http.StatusConflict)

		// 2nd call: GET to find the existing client UUID.
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/mcp-gateway/clients":
			assert.Equal(t, "existing-client", r.URL.Query().Get("clientId"))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ClientRepresentation{
				{ID: fakeClientUUID, ClientID: "existing-client"},
			})

		// 3rd call: GET to fetch the secret.
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/mcp-gateway/clients/"+fakeClientUUID+"/client-secret":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(clientSecretResponse{Value: fakeClientSecret})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	kc := NewClient(srv.URL, "mcp-gateway")
	secret, err := kc.CreateClient(context.Background(), fakeAdminToken, ClientRepresentation{
		ClientID:               "existing-client",
		Enabled:                true,
		ServiceAccountsEnabled: true,
	})

	require.NoError(t, err, "409 conflict should be handled gracefully")
	assert.Equal(t, fakeClientSecret, secret)
	assert.Equal(t, 3, callCount, "expected exactly 3 HTTP calls (create-409, find, secret)")
}

func TestDeleteClient_Success(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		switch {
		// 1st call: GET to find the client UUID.
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/mcp-gateway/clients":
			assert.Equal(t, "doomed-client", r.URL.Query().Get("clientId"))

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ClientRepresentation{
				{ID: fakeClientUUID, ClientID: "doomed-client"},
			})

		// 2nd call: DELETE the client by UUID.
		case r.Method == http.MethodDelete && r.URL.Path == "/admin/realms/mcp-gateway/clients/"+fakeClientUUID:
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	kc := NewClient(srv.URL, "mcp-gateway")
	err := kc.DeleteClient(context.Background(), fakeAdminToken, "doomed-client")

	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "expected exactly 2 HTTP calls (find, delete)")
}

func TestDeleteClient_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET returns an empty array -- client does not exist.
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/admin/realms/mcp-gateway/clients", r.URL.Path)
		assert.Equal(t, "nonexistent-client", r.URL.Query().Get("clientId"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]ClientRepresentation{})
	}))
	defer srv.Close()

	kc := NewClient(srv.URL, "mcp-gateway")
	err := kc.DeleteClient(context.Background(), fakeAdminToken, "nonexistent-client")

	require.NoError(t, err, "deleting a nonexistent client should not return an error")
}
