package runtime

import (
	"context"

	"github.com/tetratelabs/wazero"
)

// WazeroCompiledModule wraps a wazero.CompiledModule to implement types.CompiledModule
type WazeroCompiledModule struct {
	compiled wazero.CompiledModule
	name     string
}

// NewWazeroCompiledModule creates a new wrapped compiled module
func NewWazeroCompiledModule(compiled wazero.CompiledModule, name string) *WazeroCompiledModule {
	return &WazeroCompiledModule{
		compiled: compiled,
		name:     name,
	}
}

// Close releases resources associated with the compiled module
func (m *WazeroCompiledModule) Close(ctx context.Context) error {
	return m.compiled.Close(ctx)
}

// Name returns the module name
func (m *WazeroCompiledModule) Name() string {
	return m.name
}

// CompiledModule returns the underlying wazero.CompiledModule
func (m *WazeroCompiledModule) CompiledModule() wazero.CompiledModule {
	return m.compiled
}
