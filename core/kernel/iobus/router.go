package iobus

import (
	"strings"
	"sync"
)

// ============================================================================
// Router - Trie-based URI pattern matching
// ============================================================================

// Router matches URIs to drivers using a Trie (prefix tree)
type Router struct {
	root *TrieNode
	mu   sync.RWMutex
}

// TrieNode represents a node in the Trie
type TrieNode struct {
	// Driver registered at this node (if terminal)
	driver Driver

	// Exact match children (e.g., "file:", "http:")
	children map[string]*TrieNode

	// Wildcard child (matches single segment: *)
	wildcard *TrieNode

	// Globstar child (matches multiple segments: **)
	globstar *TrieNode
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		root: &TrieNode{
			children: make(map[string]*TrieNode),
		},
	}
}

// Register adds a driver to the router
//
// Pattern examples:
//   - "file://**"         matches file:///any/path
//   - "http://**"         matches http://any.host/any/path
//   - "s3://bucket/*"     matches s3://bucket/key (single segment)
//   - "kernel://runtime/onnx/*"  matches kernel://runtime/onnx/session-id
func (r *Router) Register(pattern string, driver Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Parse pattern into segments
	segments := parsePattern(pattern)

	// Traverse/create trie
	node := r.root
	for _, seg := range segments {
		switch seg {
		case "*":
			if node.wildcard == nil {
				node.wildcard = &TrieNode{
					children: make(map[string]*TrieNode),
				}
			}
			node = node.wildcard

		case "**":
			if node.globstar == nil {
				node.globstar = &TrieNode{
					children: make(map[string]*TrieNode),
				}
			}
			node = node.globstar

		default:
			if node.children[seg] == nil {
				node.children[seg] = &TrieNode{
					children: make(map[string]*TrieNode),
				}
			}
			node = node.children[seg]
		}
	}

	// Register driver at terminal node
	node.driver = driver
	return nil
}

// Match finds the best matching driver for a URI
//
// Matching rules:
//   1. Exact match beats wildcard
//   2. Longer prefix beats shorter
//   3. * beats **
func (r *Router) Match(uri string) Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	segments := parseURI(uri)
	return r.matchSegments(r.root, segments, 0)
}

// matchSegments recursively matches URI segments against the trie
func (r *Router) matchSegments(node *TrieNode, segments []string, idx int) Driver {
	// Reached end of URI
	if idx >= len(segments) {
		return node.driver
	}

	seg := segments[idx]

	// Priority 1: Try exact match first
	if child, ok := node.children[seg]; ok {
		if driver := r.matchSegments(child, segments, idx+1); driver != nil {
			return driver
		}
	}

	// Priority 2: Try wildcard (matches single segment)
	if node.wildcard != nil {
		if driver := r.matchSegments(node.wildcard, segments, idx+1); driver != nil {
			return driver
		}
	}

	// Priority 3: Try globstar (matches remaining segments)
	if node.globstar != nil {
		// Try matching rest of segments with globstar
		// Start from current position and try each subsequent position
		for i := idx; i <= len(segments); i++ {
			if driver := r.matchSegments(node.globstar, segments, i); driver != nil {
				return driver
			}
		}
	}

	return nil
}

// ============================================================================
// Pattern/URI Parsing
// ============================================================================

// parsePattern splits a pattern into segments for trie construction
//
// Examples:
//   - "file://**"                → ["file:", "**"]
//   - "http://**"                → ["http:", "**"]
//   - "s3://bucket/*/file"       → ["s3:", "bucket", "*", "file"]
//   - "kernel://runtime/onnx/*"  → ["kernel:", "runtime", "onnx", "*"]
func parsePattern(pattern string) []string {
	// Handle scheme separately (e.g., "file:", "http:")
	colonIdx := strings.Index(pattern, ":")
	if colonIdx == -1 {
		return strings.Split(pattern, "/")
	}

	scheme := pattern[:colonIdx+1] // Include the colon
	rest := pattern[colonIdx+1:]

	// Remove leading slashes
	rest = strings.TrimLeft(rest, "/")

	if rest == "" {
		return []string{scheme}
	}

	// Split rest by /
	segments := []string{scheme}
	if rest != "" {
		segments = append(segments, strings.Split(rest, "/")...)
	}

	return segments
}

// parseURI splits a URI into segments for matching
//
// Examples:
//   - "file:///tmp/test.txt"           → ["file:", "tmp", "test.txt"]
//   - "http://api.example.com/v1/data" → ["http:", "api.example.com", "v1", "data"]
//   - "s3://my-bucket/path/to/file"    → ["s3:", "my-bucket", "path", "to", "file"]
func parseURI(uri string) []string {
	// Handle scheme separately
	colonIdx := strings.Index(uri, ":")
	if colonIdx == -1 {
		return strings.Split(uri, "/")
	}

	scheme := uri[:colonIdx+1] // Include the colon
	rest := uri[colonIdx+1:]

	// Remove leading slashes
	rest = strings.TrimLeft(rest, "/")

	if rest == "" {
		return []string{scheme}
	}

	// Split rest by /
	segments := []string{scheme}
	if rest != "" {
		segments = append(segments, strings.Split(rest, "/")...)
	}

	return segments
}

// ============================================================================
// Debug/Inspection
// ============================================================================

// ListPatterns returns all registered patterns
func (r *Router) ListPatterns() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var patterns []string
	r.collectPatterns(r.root, []string{}, &patterns)
	return patterns
}

func (r *Router) collectPatterns(node *TrieNode, path []string, patterns *[]string) {
	if node.driver != nil {
		*patterns = append(*patterns, strings.Join(path, "/"))
	}

	for seg, child := range node.children {
		r.collectPatterns(child, append(path, seg), patterns)
	}

	if node.wildcard != nil {
		r.collectPatterns(node.wildcard, append(path, "*"), patterns)
	}

	if node.globstar != nil {
		r.collectPatterns(node.globstar, append(path, "**"), patterns)
	}
}

// MatchWithTrace returns the matched driver and the matching path (for debugging)
func (r *Router) MatchWithTrace(uri string) (Driver, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	segments := parseURI(uri)
	var trace []string
	driver := r.matchSegmentsWithTrace(r.root, segments, 0, &trace)
	return driver, trace
}

func (r *Router) matchSegmentsWithTrace(node *TrieNode, segments []string, idx int, trace *[]string) Driver {
	if idx >= len(segments) {
		return node.driver
	}

	seg := segments[idx]

	// Try exact match
	if child, ok := node.children[seg]; ok {
		*trace = append(*trace, seg+" (exact)")
		if driver := r.matchSegmentsWithTrace(child, segments, idx+1, trace); driver != nil {
			return driver
		}
		*trace = (*trace)[:len(*trace)-1] // Backtrack
	}

	// Try wildcard
	if node.wildcard != nil {
		*trace = append(*trace, seg+" (*)")
		if driver := r.matchSegmentsWithTrace(node.wildcard, segments, idx+1, trace); driver != nil {
			return driver
		}
		*trace = (*trace)[:len(*trace)-1] // Backtrack
	}

	// Try globstar
	if node.globstar != nil {
		for i := idx; i <= len(segments); i++ {
			consumed := segments[idx:i]
			*trace = append(*trace, strings.Join(consumed, "/")+" (**)")
			if driver := r.matchSegmentsWithTrace(node.globstar, segments, i, trace); driver != nil {
				return driver
			}
			*trace = (*trace)[:len(*trace)-1] // Backtrack
		}
	}

	return nil
}
