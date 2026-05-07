# URI Pattern Matching in io.bus

The `io.bus` driver uses sophisticated pattern matching to route resource calls to the most appropriate driver based on URI specificity.

## Pattern Syntax

URI patterns consist of three components: `scheme://host/path`

### Wildcards

- **Host wildcards**: `*.domain.com` matches any subdomain
- **Path wildcards**: `/path/to/*` matches any path with that prefix
- **Full wildcards**: `*` or `/*` matches everything

## Matching Rules

The router finds the best match using this priority:

1. **Scheme** - Must match exactly (file, https, http, fn, etc.)
2. **Host** - More specific wins:
   - Exact match (e.g., `api.example.com`) > Prefix wildcard (e.g., `*.example.com`) > Full wildcard (`*`)
3. **Path** - More specific wins:
   - Exact match (e.g., `/data/file.txt`) > Path prefix (e.g., `/data/*`) > Full wildcard (`/*`)
4. **Tie-breaker** - Fewest total wildcards wins

## Scoring System

Each match receives a specificity score (higher = more specific):

- **Host scoring:**
  - Exact match: +100 points
  - Prefix wildcard (`*.domain.com`): +50 points
  - Full wildcard (`*`): +0 points

- **Path scoring:**
  - Exact match: +100 points + (segments × 10)
  - Suffix wildcard (`/path/*`): +(segments × 10)
  - Full wildcard (`/*`): +0 points

## Examples

### File System Routing

```
Drivers registered:
1. file://*/*                     (generic file driver)
2. file:///data/*                 (data directory driver)
3. file:///data/important.txt     (specific file driver)

URI: file:///data/important.txt
├─ Driver 1 matches: score 0 (full wildcard)
├─ Driver 2 matches: score 110 (1 path segment + wildcard)
└─ Driver 3 matches: score 220 (exact match) ✓ SELECTED
```

### HTTP API Routing

```
Drivers registered:
1. https://*/*                    (generic HTTPS driver)
2. https://*.example.com/*        (domain wildcard driver)
3. https://api.example.com/*      (specific host driver)
4. https://api.example.com/v1/*   (specific API version driver)

URI: https://api.example.com/v1/users
├─ Driver 1 matches: score 0 (full wildcards)
├─ Driver 2 matches: score 50 (host wildcard)
├─ Driver 3 matches: score 100 (exact host, path wildcard)
└─ Driver 4 matches: score 120 (exact host, 2 path segments) ✓ SELECTED
```

### Subdomain Routing

```
Drivers registered:
1. https://*.example.com/*        (all subdomains)
2. https://api.example.com/*      (API subdomain)

URI: https://api.example.com/resource
├─ Driver 1 matches: score 50 (wildcard host)
└─ Driver 2 matches: score 100 (exact host) ✓ SELECTED

URI: https://admin.example.com/resource
└─ Driver 1 matches: score 50 ✓ SELECTED (only match)
```

## Benefits

1. **Multiple drivers per scheme**: Register multiple file://, https://, or fn:// drivers with different specializations
2. **Gradual specificity**: Start with generic drivers, add specific ones as needed
3. **Predictable routing**: Most specific match always wins
4. **Performance**: O(n) lookup where n = number of registered patterns
5. **Flexibility**: Drivers can overlap in their coverage areas

## Implementation Details

- Pattern matching is performed in `internal/drivers/io/bus/memory.go`
- URI parsing handles `scheme://host/path` structure
- Matching algorithm computes specificity scores
- Best match is selected based on highest score
- Ties are broken by registration order (first wins)
