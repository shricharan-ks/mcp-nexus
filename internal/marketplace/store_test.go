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

package marketplace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func newTestEntry(name, displayName, description string, category mcpv1alpha1.MarketplaceCategory, tags []string) mcpv1alpha1.MCPMarketplaceEntry {
	return mcpv1alpha1.MCPMarketplaceEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: mcpv1alpha1.MCPMarketplaceEntrySpec{
			DisplayName: displayName,
			Vendor:      "test-vendor",
			Version:     "1.0.0",
			Description: description,
			Category:    category,
			Tags:        tags,
			Source: mcpv1alpha1.MarketplaceSource{
				Image: "ghcr.io/test/" + name + ":v1",
			},
			InstallTemplate: mcpv1alpha1.InstallTemplate{
				MCPServerSpec: mcpv1alpha1.MCPServerSpec{
					Source: mcpv1alpha1.MCPServerSource{
						Image: "ghcr.io/test/" + name + ":v1",
						Port:  8080,
					},
					Protocol: mcpv1alpha1.MCPServerProtocol{
						Transport: mcpv1alpha1.TransportStreamableHTTP,
						Endpoint:  "/mcp",
					},
				},
			},
		},
	}
}

// --------------------------------------------------------------------------
// Tests
// --------------------------------------------------------------------------

func TestAdd_And_Get(t *testing.T) {
	store := NewCatalogStore()

	entry := newTestEntry("github-mcp", "GitHub MCP Server", "GitHub integration", mcpv1alpha1.CategoryDeveloperTools, nil)
	store.Add(entry)

	got, ok := store.Get("github-mcp")
	require.True(t, ok, "expected entry to be found")
	assert.Equal(t, "GitHub MCP Server", got.Spec.DisplayName)
	assert.Equal(t, "test-vendor", got.Spec.Vendor)
	assert.Equal(t, "1.0.0", got.Spec.Version)

	// Non-existent entry
	_, ok = store.Get("nonexistent")
	assert.False(t, ok, "expected entry to not be found")
}

func TestList_ByCategory(t *testing.T) {
	store := NewCatalogStore()

	store.Add(newTestEntry("github-mcp", "GitHub MCP", "GitHub integration", mcpv1alpha1.CategoryDeveloperTools, nil))
	store.Add(newTestEntry("postgres-mcp", "PostgreSQL MCP", "Database integration", mcpv1alpha1.CategoryData, nil))
	store.Add(newTestEntry("gitlab-mcp", "GitLab MCP", "GitLab integration", mcpv1alpha1.CategoryDeveloperTools, nil))

	// Filter by developer-tools
	devTools := store.List("developer-tools")
	assert.Len(t, devTools, 2, "expected 2 developer-tools entries")

	// Filter by data
	data := store.List("data")
	assert.Len(t, data, 1, "expected 1 data entry")
	assert.Equal(t, "PostgreSQL MCP", data[0].Spec.DisplayName)

	// Empty category returns all
	all := store.List("")
	assert.Len(t, all, 3, "expected 3 total entries")

	// Non-matching category
	empty := store.List("security")
	assert.Len(t, empty, 0, "expected 0 entries for unmatched category")
}

func TestSearch(t *testing.T) {
	store := NewCatalogStore()

	store.Add(newTestEntry("github-mcp", "GitHub MCP Server", "Manage repositories and issues", mcpv1alpha1.CategoryDeveloperTools, []string{"git", "vcs"}))
	store.Add(newTestEntry("postgres-mcp", "PostgreSQL MCP", "Database queries and management", mcpv1alpha1.CategoryData, []string{"database", "sql"}))
	store.Add(newTestEntry("slack-mcp", "Slack MCP", "Team communication integration", mcpv1alpha1.CategoryCommunication, []string{"chat", "messaging"}))

	// Match by DisplayName
	results := store.Search("GitHub")
	require.Len(t, results, 1)
	assert.Equal(t, "github-mcp", results[0].Name)

	// Match by Description (case-insensitive)
	results = store.Search("database")
	require.Len(t, results, 1)
	assert.Equal(t, "postgres-mcp", results[0].Name)

	// Match by Tag
	results = store.Search("chat")
	require.Len(t, results, 1)
	assert.Equal(t, "slack-mcp", results[0].Name)

	// Case-insensitive match
	results = store.Search("GITHUB")
	require.Len(t, results, 1)
	assert.Equal(t, "github-mcp", results[0].Name)

	// No match
	results = store.Search("nonexistent-tool")
	assert.Len(t, results, 0)
}

func TestDelete(t *testing.T) {
	store := NewCatalogStore()

	store.Add(newTestEntry("github-mcp", "GitHub MCP", "GitHub integration", mcpv1alpha1.CategoryDeveloperTools, nil))
	store.Add(newTestEntry("postgres-mcp", "PostgreSQL MCP", "Database integration", mcpv1alpha1.CategoryData, nil))

	assert.Equal(t, 2, store.Count())

	store.Delete("github-mcp")

	assert.Equal(t, 1, store.Count())

	_, ok := store.Get("github-mcp")
	assert.False(t, ok, "expected deleted entry to not be found")

	// Delete non-existent entry should not panic
	store.Delete("nonexistent")
	assert.Equal(t, 1, store.Count())
}

func TestCount(t *testing.T) {
	store := NewCatalogStore()

	assert.Equal(t, 0, store.Count(), "empty store should have count 0")

	store.Add(newTestEntry("entry-1", "Entry 1", "First", mcpv1alpha1.CategoryCustom, nil))
	assert.Equal(t, 1, store.Count())

	store.Add(newTestEntry("entry-2", "Entry 2", "Second", mcpv1alpha1.CategoryCustom, nil))
	assert.Equal(t, 2, store.Count())

	// Overwriting existing entry should not change count
	store.Add(newTestEntry("entry-1", "Entry 1 Updated", "Updated", mcpv1alpha1.CategoryCustom, nil))
	assert.Equal(t, 2, store.Count())

	store.Delete("entry-1")
	assert.Equal(t, 1, store.Count())
}
