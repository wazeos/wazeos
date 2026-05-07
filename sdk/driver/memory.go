package driver

import (
	"unsafe"
)

var (
	// memoryBuffer is used for temporary allocations
	memoryBuffer []byte
)

// getWASMMemory returns a slice view of the entire WASM linear memory.
// In WASM, all memory starts at address 0, so we create an unsafe slice from there.
func getWASMMemory() []byte {
	// Get the first byte of memory (address 0)
	// WASM memory is contiguous from 0 to current size
	// We'll use a large enough size - the actual memory size is managed by the runtime
	const maxMemSize = 128 * 1024 * 1024 // 128MB max for safety
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(0))), maxMemSize)
}

// Allocate reserves memory and returns a pointer to it.
func Allocate(size uint32) uint32 {
	memoryBuffer = make([]byte, size)
	return uint32(uintptr(unsafe.Pointer(&memoryBuffer[0])))
}

// Deallocate frees previously allocated memory.
func Deallocate() {
	memoryBuffer = nil
}

// GetMemory returns the current memory buffer for reading/writing data.
func GetMemory() []byte {
	return memoryBuffer
}

// CopyFromMemory copies data from WASM linear memory at the given absolute address.
func CopyFromMemory(ptr, length uint32) []byte {
	mem := getWASMMemory()
	data := make([]byte, length)
	copy(data, mem[ptr:ptr+length])
	return data
}

// CopyToMemory copies data to the memory buffer and returns the pointer.
func CopyToMemory(data []byte) uint32 {
	ptr := Allocate(uint32(len(data)))
	copy(GetMemory(), data)
	return ptr
}

// ReadString reads a string from WASM linear memory at the given absolute address.
func ReadString(ptr, length uint32) string {
	mem := getWASMMemory()
	return string(mem[ptr : ptr+length])
}
