# App SDK Error Messages

This document shows what users will see when encountering common errors.

## Missing Driver Errors

When a required driver isn't installed, the App SDK provides clear, actionable error messages.

### File Driver Not Installed

**Code:**
```go
data, err := ctx.IO.ReadFile("/tmp/data.txt")
```

**Error message:**
```
FILE_READ_ERROR: no driver found for URI: file:///tmp/data.txt
```

**What to do:** Install the file driver (`io.resource.file`)

### HTTP Driver Not Installed

**Code:**
```go
resp, err := ctx.IO.Get("https://api.example.com/data")
```

**Error message:**
```
HTTP_REQUEST_ERROR: no driver found for URI: https://api.example.com/data
```

**What to do:** Install the HTTP driver (`io.resource.http`)

### Queue/IPC Driver Not Installed

**Code:**
```go
err := ctx.IO.Publish("events.orders", message)
```

**Error message:**
```
QUEUE_PUBLISH_ERROR: no driver found for URI: queue://events.orders
```

**What to do:** Install the IPC/queue driver (`io.ipc` or `io.resource.queue`)

### App-to-App Driver Not Installed

**Code:**
```go
result, err := ctx.IO.CallApp("user-service", "get", "123")
```

**Error message:**
```
APP_CALL_ERROR: no driver found for URI: fn://user-service/get/123
```

**What to do:** Install the function call driver (`io.resource.fn`)

## Other Common Errors

### File Not Found

**Error message:**
```
FILE_READ_ERROR: file not found: /tmp/missing.txt
```

### Permission Denied

**Error message:**
```
FILE_WRITE_ERROR: permission denied: /etc/protected.conf
```

### HTTP 404

**Error message:**
```
HTTP_REQUEST_ERROR: 404 Not Found
```

### Invalid Input

**Error message:**
```
INVALID_INPUT: invalid JSON in request body
```

## Error Message Format

All App SDK errors follow this format:

```
ERROR_CODE: detailed error message
```

- **ERROR_CODE**: Identifies the type of error (e.g., `FILE_READ_ERROR`, `HTTP_REQUEST_ERROR`)
- **detailed error message**: Explains what went wrong, including the URI or resource that failed

## Implementation Details

The App SDK automatically extracts the most informative error message from the kernel's response:

```go
// Helper function in io.go
func getErrorMessage(result *driver.ResourceResult) string {
    // If Body contains a more detailed message, use it
    if len(result.Body) > 0 {
        bodyMsg := string(result.Body)
        if bodyMsg != result.Error && len(bodyMsg) > len(result.Error) {
            return bodyMsg
        }
    }
    return result.Error
}
```

This ensures that when the kernel provides detailed error messages (like "no driver found for URI: file:///path"), users see them instead of generic errors like "resource not found".
