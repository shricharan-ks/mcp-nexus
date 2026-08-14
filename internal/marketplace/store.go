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
	"strings"
	"sync"

	mcpv1alpha1 "github.com/mcp-gateway/mcp-gateway/api/v1alpha1"
)

// CatalogStore is a thread-safe in-memory store for marketplace catalog entries.
// It can be replaced with a persistent backend (e.g. PostgreSQL) later.
type CatalogStore struct {
	mu      sync.RWMutex
	entries map[string]mcpv1alpha1.MCPMarketplaceEntry
}

// NewCatalogStore creates a new empty CatalogStore.
func NewCatalogStore() *CatalogStore {
	return &CatalogStore{
		entries: make(map[string]mcpv1alpha1.MCPMarketplaceEntry),
	}
}

// Add inserts or updates an entry in the catalog, keyed by its metadata name.
func (s *CatalogStore) Add(entry mcpv1alpha1.MCPMarketplaceEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.Name] = entry
}

// Get retrieves an entry by name. Returns the entry and true if found,
// or a zero value and false otherwise.
func (s *CatalogStore) Get(name string) (mcpv1alpha1.MCPMarketplaceEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[name]
	return entry, ok
}

// List returns all entries matching the given category. If category is empty,
// all entries are returned.
func (s *CatalogStore) List(category string) []mcpv1alpha1.MCPMarketplaceEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []mcpv1alpha1.MCPMarketplaceEntry
	for _, entry := range s.entries {
		if category == "" || string(entry.Spec.Category) == category {
			result = append(result, entry)
		}
	}
	return result
}

// Search performs a case-insensitive substring match against DisplayName,
// Description, and Tags. Returns all matching entries.
func (s *CatalogStore) Search(query string) []mcpv1alpha1.MCPMarketplaceEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	q := strings.ToLower(query)
	var result []mcpv1alpha1.MCPMarketplaceEntry
	for _, entry := range s.entries {
		if matchesQuery(entry, q) {
			result = append(result, entry)
		}
	}
	return result
}

// Delete removes an entry from the catalog by name.
func (s *CatalogStore) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, name)
}

// Count returns the number of entries in the catalog.
func (s *CatalogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// matchesQuery checks if any of the entry's searchable fields contain the query
// string (case-insensitive).
func matchesQuery(entry mcpv1alpha1.MCPMarketplaceEntry, query string) bool {
	if strings.Contains(strings.ToLower(entry.Spec.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(entry.Spec.Description), query) {
		return true
	}
	for _, tag := range entry.Spec.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
