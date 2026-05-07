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

Each match receives a specificity score (higher = more specific). The scoring considers both segment count and actual length for fine-grained matching.

- **Host scoring:**
  - Exact match: (segments × 15) + length + 50 points
    - Example: `api.example.com` = (3 × 15) + 15 + 50 = 110
  - Prefix wildcard (`*.domain.com`): (suffix segments × 15) points
    - Example: `*.example.com` = (2 × 15) = 30
    - Example: `*.com` = (1 × 15) = 15
  - Full wildcard (`*`): 0 points

- **Path scoring:**
  - Exact match: (segments × 10) + length + 100 points
    - Example: `/data/file.txt` = (2 × 10) + 15 + 100 = 135
  - Suffix wildcard (`/path/*`): (prefix segments × 10) + prefix length
    - Example: `/data/users/*` = (2 × 10) + 12 = 32
    - Example: `/data/*` = (1 × 10) + 6 = 16
  - Full wildcard (`/*`): 0 points

**Key improvement**: Patterns with the same segment count are differentiated by length. For example, `/abc/*` scores higher than `/a/*` because the prefix is longer (3 vs 1 character).

## Examples

### File System Routing

```
Drivers registered:
1. file://*/*                     (generic file driver)
2. file:///data/*                 (data directory driver)
3. file:///data/important.txt     (specific file driver)

URI: file:///data/important.txt
├─ Driver 1 matches: score 0 (full wildcard)
├─ Driver 2 matches: score 80 (1 segment × 10 + 6 chars = 16)
└─ Driver 3 matches: score 204 (2 segments × 10 + 24 chars + 100) ✓ SELECTED
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
├─ Driver 2 matches: score 30 (2 host segments × 15)
├─ Driver 3 matches: score 110 (exact host: 3×15 + 15 chars + 50)
└─ Driver 4 matches: score 133 (exact host + 1 segment × 10 + 4 chars) ✓ SELECTED
```

### Subdomain Routing

```
Drivers registered:
1. https://*.example.com/*        (all subdomains)
2. https://api.example.com/*      (API subdomain)

URI: https://api.example.com/resource
├─ Driver 1 matches: score 30 (wildcard host: 2 segments × 15)
└─ Driver 2 matches: score 110 (exact host) ✓ SELECTED

URI: https://admin.example.com/resource
└─ Driver 1 matches: score 30 ✓ SELECTED (only match)
```

### Length-Based Granularity

```
Drivers registered:
1. file:///a/*                    (short path)
2. file:///abc/*                  (longer path)

URI: file:///abc/test.txt
├─ Driver 1 matches: score 77 (1 segment × 10 + 1 char + 6 chars)
└─ Driver 2 matches: score 79 (1 segment × 10 + 3 chars + 6 chars) ✓ SELECTED

Even with the same segment count, the longer prefix wins!
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
