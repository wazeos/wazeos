# Error Handling Improvement

## Summary

Improved the App SDK's error handling to provide clear, actionable error messages when drivers are missing or operations fail.

## The Problem

**Before:** When a driver wasn't installed, users saw generic, unhelpful errors:

```
FILE_READ_ERROR: resource not found
```

This didn't tell users:
- What driver was missing
- What URI pattern wasn't matched
- How to fix the problem

## The Solution

**After:** Users now see detailed, actionable error messages:

```
FILE_READ_ERROR: no driver found for URI: file:///tmp/data.txt
```

This clearly indicates:
- A driver is missing (not just a file not found)
- The exact URI pattern that needs a driver (`file://`)
- What driver to install (`io.resource.file`)

## Implementation

Added a helper function `getErrorMessage()` that extracts the best error message from resource call results:

```go
// getErrorMessage extracts the best error message from a ResourceResult.
// Prefers the Body field when it contains more detailed information.
func getErrorMessage(result *driver.ResourceResult) string {
    // If Body contains a more detailed message, use it
    if len(result.Body) > 0 {
        bodyMsg := string(result.Body)
        // Use Body if it's different and more informative than Error
        if bodyMsg != result.Error && len(bodyMsg) > len(result.Error) {
            return bodyMsg
        }
    }
    return result.Error
}
```

This is now used in all I/O operations:
- `ReadFile()`, `WriteFile()`, `DeleteFile()`, `ListFiles()`
- `Publish()`, `Consume()`

## Examples

### Missing File Driver

```go
data, err := ctx.IO.ReadFile("/tmp/config.json")
// Error: FILE_READ_ERROR: no driver found for URI: file:///tmp/config.json
```

**Action:** Install `io.resource.file` driver

### Missing HTTP Driver

```go
resp, err := ctx.IO.Get("https://api.example.com/users")
// Error: HTTP_REQUEST_ERROR: no driver found for URI: https://api.example.com/users
```

**Action:** Install `io.resource.http` driver

### Missing Queue Driver

```go
err := ctx.IO.Publish("events.orders", message)
// Error: QUEUE_PUBLISH_ERROR: no driver found for URI: queue://events.orders
```

**Action:** Install IPC/queue driver

### Missing Fn Driver

```go
result, err := ctx.IO.CallApp("user-service", "get", "123")
// Error: APP_CALL_ERROR: no driver found for URI: fn://user-service/get/123
```

**Action:** Install `io.resource.fn` driver

## Benefits

1. **Clear diagnosis** - Users immediately know what's wrong
2. **Actionable** - Error message tells them what driver to install
3. **Debuggable** - Full URI shown for troubleshooting
4. **Consistent** - All I/O operations follow the same pattern

## Files Modified

- [sdk/app/io.go](io.go) - Added `getErrorMessage()` helper and updated all error handling
- [sdk/app/README.md](README.md) - Documented error handling behavior
- [sdk/app/ERROR_EXAMPLES.md](ERROR_EXAMPLES.md) - Comprehensive error message examples
