package tools

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
)

// Severity classifies a Check as required (Hard) or advisory (Soft). Hard
// failures exit the doctor with non-zero status; soft failures are warnings.
type Severity int

const (
	SeverityHard Severity = iota
	SeveritySoft
)

func (s Severity) String() string {
	switch s {
	case SeverityHard:
		return "hard"
	case SeveritySoft:
		return "soft"
	}
	return "unknown"
}

// Check is a single doctor probe. Checks register themselves from
// package-level init(); the doctor tool iterates the registry.
//
// Order is preserved as registered, with Hard checks always running before
// Soft ones regardless of registration order.
type Check struct {
	Name     string
	Desc     string
	Severity Severity
	// Gate, if non-empty, names a Bool param on the doctor tool that must be
	// set to true for this check to run (e.g. "ping").
	Gate string
	Run  func(ctx context.Context) (detail string, err error)
}

func (c Check) validate() error {
	if c.Name == "" {
		return fmt.Errorf("check: empty Name")
	}
	if c.Run == nil {
		return fmt.Errorf("check %q: nil Run", c.Name)
	}
	// Gate becomes a Bool flag on the doctor tool — auto-derived in
	// gateParams. Reject names that wouldn't survive Param validation.
	if c.Gate != "" && !isSnakeCase(c.Gate) {
		return fmt.Errorf("check %q: Gate %q must be lower_snake_case", c.Name, c.Gate)
	}
	return nil
}

var (
	checkMu sync.RWMutex
	checks  []Check
	checked = map[string]struct{}{}
)

// RegisterCheck adds a Check to the registry. Called from package-level
// init(). Panics on invalid definition or duplicate Name.
func RegisterCheck(c Check) {
	if err := c.validate(); err != nil {
		panic(err)
	}
	checkMu.Lock()
	defer checkMu.Unlock()
	if _, dup := checked[c.Name]; dup {
		panic(fmt.Errorf("check %q registered twice", c.Name))
	}
	checked[c.Name] = struct{}{}
	checks = append(checks, c)
}

// Checks returns every registered check sorted by severity (hard before
// soft); slices.SortStableFunc preserves the registration order within a
// severity.
func Checks() []Check {
	checkMu.RLock()
	defer checkMu.RUnlock()
	out := slices.Clone(checks)
	slices.SortStableFunc(out, func(a, b Check) int {
		return cmp.Compare(a.Severity, b.Severity)
	})
	return out
}
