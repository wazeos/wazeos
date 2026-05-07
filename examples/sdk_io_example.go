package main

import (
	"fmt"

	"github.com/wazeos/wazeos/sdk/app"
)

func main() {
	app.Run(handler)
}

func handler(ctx *app.Context, req *app.Request) (*app.Response, error) {
	// File operations
	fileResult, err := ctx.IO("file:///tmp/config.txt", []string{"read"}).Call(nil)
	if err != nil {
		return nil, err
	}
	ctx.Log.Info("read file", app.Any("result", fileResult))

	// Write a file
	err = ctx.IO("file:///tmp/output.txt", []string{"write"}).Call(map[string]interface{}{
		"data": []byte("Hello, WazeOS!"),
	})
	if err != nil {
		return nil, err
	}

	// HTTP GET request
	httpResult, err := ctx.IO("https://api.example.com/data", []string{"GET"}).Call(nil)
	if err != nil {
		return nil, err
	}
	ctx.Log.Info("http response", app.Any("result", httpResult))

	// HTTP POST request with headers
	postResult, err := ctx.IO("https://api.example.com/data", []string{"POST"}).Call(map[string]interface{}{
		"body":    []byte(`{"key": "value"}`),
		"headers": map[string]string{"Content-Type": "application/json"},
	})
	if err != nil {
		return nil, err
	}
	ctx.Log.Info("post response", app.Any("result", postResult))

	// Call another app
	appResult, err := ctx.IO("fn://wazeos/logger", []string{"invoke"}).Call(map[string]interface{}{
		"level":   "info",
		"message": "Hello from app!",
	})
	if err != nil {
		return nil, err
	}
	ctx.Log.Info("app call result", app.Any("result", appResult))

	// Queue operations - publish
	err = ctx.IO("queue://events", []string{"write"}).Call(map[string]interface{}{
		"message": []byte("event data"),
		"key":     "event-key",
	})
	if err != nil {
		return nil, err
	}

	// Queue operations - consume
	queueResult, err := ctx.IO("queue://events", []string{"read"}).Call(map[string]interface{}{
		"maxCount": 10,
		"timeout":  5,
		"group":    "consumer-group",
	})
	if err != nil {
		return nil, err
	}
	ctx.Log.Info("consumed messages", app.Any("result", queueResult))

	return app.Success([]byte("All operations completed successfully!")), nil
}
