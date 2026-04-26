package tools

import (
	"fmt"
	"maps"
	"slices"
	"sync"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Tool{}
)

// Register adds a tool to the global registry. Called from package-level
// init() in each tool file. Panics on invalid definition or duplicate Name —
// these are programmer errors caught at import time.
func Register(t Tool) {
	if err := t.validate(); err != nil {
		panic(err)
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := reg[t.Name]; dup {
		panic(fmt.Errorf("tool %q registered twice", t.Name))
	}
	reg[t.Name] = t
}

// All returns every registered tool sorted alphabetically by Name. The order
// is stable so prompt rendering and `--help` output do not flap.
func All() []Tool {
	regMu.RLock()
	defer regMu.RUnlock()
	names := slices.Sorted(maps.Keys(reg))
	out := make([]Tool, 0, len(names))
	for _, n := range names {
		out = append(out, reg[n])
	}
	return out
}

// Get returns the tool with the given Name and whether it exists.
func Get(name string) (Tool, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	t, ok := reg[name]
	return t, ok
}

// MustGet returns the tool with the given Name or panics. Used in tests.
func MustGet(name string) Tool {
	t, ok := Get(name)
	if !ok {
		panic(fmt.Errorf("tool %q not registered", name))
	}
	return t
}
