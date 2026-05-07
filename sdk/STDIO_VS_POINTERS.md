# STDIN/STDOUT vs Pointer-Based Communication

This document compares the two approaches for host-WASM communication in WazeOS drivers.

## Comparison Table

| Aspect | Pointer-Based | STDIN/STDOUT |
|--------|---------------|--------------|
| **Safety** | ⚠️ Unsafe pointer arithmetic | ✅ No pointers exposed |
| **Complexity** | ⚠️ Manual memory management | ✅ Standard I/O |
| **Debugging** | ⚠️ Hard to inspect memory | ✅ Easy to test with CLI |
| **Portability** | ⚠️ WASM-specific | ✅ Standard WASI |
| **Error Handling** | ⚠️ Can corrupt memory | ✅ Clean errors |
| **Testing** | ⚠️ Need host runtime | ✅ Test with echo/pipe |
| **Performance** | ✅ ~5µs per call | ⚠️ ~20µs per call |
| **Memory Copies** | ✅ 1 copy | ⚠️ 2 copies |

## Pointer-Based Approach (Legacy)

### How It Works

```
┌─────────────┐              ┌─────────────┐
│    Host     │              │    WASM     │
│  (Go/wazero)│              │  (TinyGo)   │
└──────┬──────┘              └──────┬──────┘
       │                            │
       │ 1. Write to linear memory  │
       │ (call at offset 1000)      │
       ├────────────────────────────>
       │                            │
       │ 2. Call: handle_call(1000, 50)
       ├────────────────────────────>
       │                            │
       │                     3. Read from memory[1000:1050]
       │                     4. Process
       │                     5. Write result to memory[2000:]
       │                            │
       │ 6. Return pointer: 2000    │
       <────────────────────────────┤
       │                            │
  7. Read from memory[2000:]        │
       │                            │
```

### Code Example

```go
// Driver code (SCARY!)
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    // Read from raw memory address
    callData := memoryBuffer[callPtr : callPtr+callLen]
    var call ResourceCall
    json.Unmarshal(callData, &call)

    // Process...
    result := process(&call)

    // Write to raw memory address
    resultJSON, _ := json.Marshal(result)
    resultPtr := allocate(uint32(len(resultJSON)))
    copy(memoryBuffer, resultJSON)

    return resultPtr  // Return raw pointer!
}
```

### Safety Issues

1. **Buffer Overruns**: Wrong offset can read/write wrong memory
2. **Dangling Pointers**: Memory freed before host reads it
3. **Type Confusion**: Interpreting data as wrong type
4. **Memory Leaks**: Forgetting to deallocate
5. **Race Conditions**: Concurrent access to shared memory
6. **Debugging Nightmares**: Can't easily inspect what's in memory

### When It Makes Sense

- Performance-critical drivers (< 20µs overhead requirement)
- Large data transfers (> 10MB)
- Streaming data
- Low-level drivers (custom binary protocols)

## STDIN/STDOUT Approach (Recommended)

### How It Works

```
┌─────────────┐              ┌─────────────┐
│    Host     │              │    WASM     │
│  (Go/wazero)│              │  (TinyGo)   │
└──────┬──────┘              └──────┬──────┘
       │                            │
       │ 1. Write JSON to stdin     │
       ├────────────────────────────>
       │                            │
       │                      2. Read from stdin
       │                      3. Parse JSON
       │                      4. Process
       │                      5. Write JSON to stdout
       │                            │
       │ 6. Read from stdout        │
       <────────────────────────────┤
       │                            │
       │ 7. Parse JSON              │
       │                            │
```

### Code Example

```go
// Driver code (SAFE!)
func main() {
    handler := &MyDriver{}

    // Option 1: Handle one request and exit
    driver.ServeResourceOnce(handler)

    // Option 2: Long-running mode
    // driver.ServeResourceStdio(handler)
}

// Your handler - no pointers!
func (d *MyDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    // Just business logic
    data, err := os.ReadFile(call.URI)
    if err != nil {
        return driver.NewErrorResult(404, "not found"), nil
    }
    return driver.NewResourceResult(200, data), nil
}
```

### Safety Benefits

1. ✅ **No Pointers**: Never see or manipulate memory addresses
2. ✅ **Type Safe**: JSON deserialization catches type errors
3. ✅ **Easy Testing**: Test drivers with command-line tools
4. ✅ **Clear Errors**: I/O errors are explicit
5. ✅ **Standard Protocol**: Uses well-tested stdin/stdout
6. ✅ **Debuggable**: Can inspect JSON with `echo | driver | jq`

### Example: Testing from Command Line

```bash
# Build driver
tinygo build -o driver.wasm -target=wasi main.go

# Test with wasmtime (no host needed!)
echo '{"uri":"file:///tmp/test.txt","method":"READ"}' | \
  wasmtime driver.wasm | \
  jq .

# Output:
{
  "statusCode": 200,
  "headers": {"Content-Type": "application/octet-stream"},
  "body": "ZmlsZSBjb250ZW50cw=="
}
```

### Performance Overhead

**Typical Request**:
- Pointer-based: 5-10µs
- STDIN/STDOUT: 15-25µs
- **Overhead: +10-15µs** (~2-3x)

**But**:
- File I/O: 50-500µs
- Network I/O: 500-5000µs
- Database query: 1000-10000µs

**Conclusion**: The 15µs overhead is negligible compared to actual I/O operations.

## Invocation Models

### Single-Shot (Recommended for STDIN/STDOUT)

Each request spawns a new WASM instance:

```go
func main() {
    driver.ServeResourceOnce(&MyDriver{})
}
```

**Pros**:
- ✅ Complete isolation between requests
- ✅ No state management needed
- ✅ Can't leak memory across requests
- ✅ Simpler to reason about

**Cons**:
- ⚠️ WASM instantiation overhead (~1ms)
- ⚠️ Can't cache connections/state

**Best for**: Stateless drivers (file, HTTP client, auth)

### Long-Running (Optional for STDIN/STDOUT)

Single WASM instance handles multiple requests:

```go
func main() {
    driver.ServeResourceStdio(&MyDriver{})
}
```

**Pros**:
- ✅ No instantiation overhead
- ✅ Can maintain connections/caches

**Cons**:
- ⚠️ Need to manage state carefully
- ⚠️ Memory leaks accumulate
- ⚠️ Errors can corrupt state

**Best for**: Stateful drivers (database connections, caches)

## Migration Guide

### From Pointer-Based to STDIN/STDOUT

**Step 1**: Update imports

```diff
  import "github.com/wazeos/wazeos/sdk/driver"
```

**Step 2**: Remove pointer exports

```diff
- //export handle_call
- func handleCall(callPtr, callLen uint32) uint32 {
-     return driver.ServeResource(handler, callPtr, callLen)
- }
```

**Step 3**: Use stdio serve

```diff
  func main() {
+     handler := &MyDriver{}
+     driver.ServeResourceOnce(handler)
  }
```

**Step 4**: Update logging

```diff
- fmt.Println("Driver initialized")  // Goes to stdout!
+ fmt.Fprintln(os.Stderr, "Driver initialized")  // Use stderr
```

### Host-Side Changes (Future)

The host (RuntimeExec) needs to support both modes:

```go
// Pointer-based (current)
result := wasmModule.ExportedFunction("handle_call").Call(callPtr, callLen)

// STDIN/STDOUT (new)
stdin.Write(callJSON)
stdout.Read(resultJSON)
```

## Decision Matrix

### Use STDIN/STDOUT When:

- ✅ Safety is more important than performance
- ✅ Driver is stateless
- ✅ You want easy CLI testing
- ✅ Performance overhead < 50µs is acceptable
- ✅ You're new to WASM development

### Use Pointers When:

- ⚠️ Performance is critical (< 20µs per call)
- ⚠️ Handling large data (> 10MB)
- ⚠️ Need custom binary protocol
- ⚠️ Have experienced WASM developers
- ⚠️ Can invest in extensive testing

## SDK Support

Both approaches are supported:

```go
// Pointer-based
//export handle_call
func handleCall(callPtr, callLen uint32) uint32 {
    return driver.ServeResource(handler, callPtr, callLen)
}

// STDIN/STDOUT
func main() {
    driver.ServeResourceOnce(handler)  // Single-shot
    // OR
    driver.ServeResourceStdio(handler) // Long-running
}
```

## Example: Complete Safe Driver

```go
package main

import (
    "os"
    "github.com/wazeos/wazeos/sdk/driver"
)

type FileDriver struct{}

func (d *FileDriver) HandleCall(call *driver.ResourceCall) (*driver.ResourceResult, error) {
    switch call.Method {
    case "READ":
        data, err := os.ReadFile(call.URI)
        if err != nil {
            return driver.NewErrorResult(404, "not found"), nil
        }
        return driver.NewResourceResult(200, data), nil
    default:
        return driver.NewErrorResult(405, "method not allowed"), nil
    }
}

func main() {
    // That's it! No pointers, no unsafe code!
    driver.ServeResourceOnce(&FileDriver{})
}
```

## Conclusion

**Recommendation**: Use STDIN/STDOUT for all new drivers unless you have specific performance requirements that justify the added complexity and safety risks of pointer-based communication.

The ~15µs overhead is negligible compared to actual I/O operations, and the safety and debugging benefits far outweigh the performance cost.

## Future Improvements

1. **Hybrid Mode**: Use stdin/stdout for small requests, pointers for large transfers
2. **Streaming**: Support chunked I/O for large files
3. **Multiplexing**: Handle multiple concurrent requests over same stdin/stdout
4. **Binary Protocol**: Use msgpack instead of JSON for better performance
5. **Zero-Copy**: Map files directly into WASM memory (when safe)
