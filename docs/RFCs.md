# WazeOS v2 RFCs (Requests for Comments)

This document tracks all RFCs for WazeOS v2 architectural and design decisions.

## Active RFCs

### Implemented ✅

| RFC | Title | Status | Author | Date |
|-----|-------|--------|--------|------|
| [RFC-001](./RFC-001-driver-architecture.md) | Driver Architecture | ✅ Implemented | WazeOS Core Team | 2026-05-08 |
| [RFC-002](./RFC-002-handle-system.md) | Handle System | ✅ Implemented | WazeOS Core Team | 2026-05-08 |

**RFC-001: Driver Architecture**
- **Problem**: v1 had unclear driver taxonomy and implicit capabilities
- **Solution**: 4 driver classes (io.connect, io.listen, runtime.*, kernel.*) with explicit capability declarations
- **Impact**: Clear driver model, Trie-based routing (O(log n)), self-registration
- **Status**: Implemented in v2 kernel

**RFC-002: Handle System**
- **Problem**: v1 sent 20MB models with every request (2GB for 100 inferences)
- **Solution**: Load once, reference by handle ID (kernel://session/{uuid})
- **Impact**: 100x reduction in data transfer, 95% memory savings, constant WASM memory
- **Status**: Implemented in session manager

### Under Review 📝

| RFC | Title | Status | Author | Date |
|-----|-------|--------|--------|------|
| [RFC-003](./RFC-003-binary-protocol.md) | Binary Protocol | 📝 TODO | - | - |
| [RFC-004](./RFC-004-security-model.md) | Security Model | 📝 TODO | - | - |
| [RFC-005](./RFC-005-cli-tooling.md) | CLI Tooling | 📝 TODO | - | - |

### Planned 📋

| RFC | Title | Description | Priority |
|-----|-------|-------------|----------|
| RFC-003 | Binary Protocol | MessagePack vs Cap'n Proto, protocol negotiation | High |
| RFC-004 | Security Model | Permission inheritance, audit logging, credentials | High |
| RFC-005 | CLI Tooling | Agent-native CLI commands, JSON output mode | High |
| RFC-006 | MCP Integration | Tool registration, schema validation | Medium |
| RFC-007 | Streaming API | Backpressure, chunking, flow control | Medium |
| RFC-008 | Plugin System | Third-party extensions, hooks | Low |

## Rejected RFCs ❌

| RFC | Title | Reason | Date |
|-----|-------|--------|------|
| - | - | - | - |

_(No rejected RFCs yet)_

## RFC Process

### When to Write an RFC

Write an RFC for:
- ✅ Architectural changes (new driver classes, protocol changes)
- ✅ API changes (breaking changes, new interfaces)
- ✅ Security changes (permission model, auth/authz)
- ✅ Performance changes (memory model, concurrency)

Don't write an RFC for:
- ❌ Bug fixes (same behavior)
- ❌ Documentation updates
- ❌ Test additions
- ❌ Internal refactoring

### RFC Template

```markdown
# RFC-XXX: [Title]

**Status**: Draft | Under Review | Accepted | Rejected | Implemented
**Author**: [Name]
**Created**: [Date]
**Updated**: [Date]

## Abstract
[1-2 paragraph summary]

## Motivation
**Problem**: What problem are we solving?
**Why now**: Why is this change necessary?
**Impact**: Who is affected?

## Proposed Solution
**Design**: How will we solve this?
**Alternatives considered**: What else did we evaluate?
**Why this approach**: Why is this best?

## Detailed Design
[Technical details, diagrams, code examples]

## Migration Path
**Breaking changes**: What breaks?
**Migration steps**: How do users migrate?

## Trade-offs
**Pros**: Benefits
**Cons**: Drawbacks
**Risks**: What could go wrong?

## Decision
**Status**: Why accepted/rejected?
**Date**: When?
**Reviewers**: Who approved?
```

### Review Timeline

- **Draft**: Author creates RFC
- **Review**: Minimum 3 days for feedback
- **Decision**: Accept or reject with reasoning
- **Implementation**: Assign owner, track progress
- **Completion**: Update RFC with lessons learned

### Creating an RFC

```bash
# Quick RFC (small changes)
$ wazeos rfc new "Add --quiet flag" --quick

# Full RFC (major changes)
$ wazeos rfc new "Distributed Handles" --full

# List all RFCs
$ wazeos rfc list

# View RFC status
$ wazeos rfc status RFC-003
```

## RFC Quick Reference

### Accepted Design Patterns (from RFCs)

Based on accepted RFCs, these patterns should be followed:

**From RFC-001 (Driver Architecture)**:
- ✅ Use driver classes: io.connect, io.listen, runtime.*, kernel.*
- ✅ Declare capabilities explicitly: CapCall, CapStream, CapHandle
- ✅ Self-register drivers via init()
- ✅ Use Trie for URI routing (O(log n))

**From RFC-002 (Handle System)**:
- ✅ Use handles for stateful resources (models, connections)
- ✅ Handle ID format: kernel://session/{uuid}
- ✅ Automatic cleanup: GC + reference counting
- ✅ Permission inheritance: handles inherit creator's permissions

### Common RFC Patterns

When writing an RFC, include:

1. **Evidence**: Benchmarks, measurements, data
2. **Alternatives**: Why not approach X, Y, or Z?
3. **Migration**: How do users transition?
4. **Code examples**: Show both old and new way
5. **Trade-offs**: Honest pros/cons analysis

### RFC Review Checklist

Before approving an RFC:
- [ ] Problem is clearly stated
- [ ] Solution addresses the problem
- [ ] Alternatives were considered
- [ ] Migration path is clear
- [ ] Trade-offs are documented
- [ ] Code examples are provided
- [ ] Breaking changes are identified
- [ ] Performance impact is measured

## Contributing

To propose a change:
1. Read existing RFCs to understand current design
2. Check if similar RFC exists (accepted or rejected)
3. Write RFC following template
4. Submit for review (minimum 3 days)
5. Address feedback and update RFC
6. Wait for decision (accept/reject)

---

**Last Updated**: 2026-05-08
**Total RFCs**: 2 implemented, 3 planned, 0 rejected
