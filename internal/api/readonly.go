package api

import "fmt"

// Write commands on the Raven local API allowlist. Perch v1 must never invoke these.
var writeCommands = map[string]struct{}{
	"set":   {},
	"unset": {},
	"new":   {},
	"move":  {},
}

// daily resolves or creates a daily note. Creating is a write; Perch does not call it.
var createCommands = map[string]struct{}{
	"daily": {},
}

// IsWrite reports whether commandID mutates the vault through the local API.
func IsWrite(commandID string) bool {
	_, ok := writeCommands[commandID]
	return ok
}

// IsForbidden reports whether Perch v1 must refuse to invoke commandID.
func IsForbidden(commandID string) bool {
	if IsWrite(commandID) {
		return true
	}
	_, ok := createCommands[commandID]
	return ok
}

func rejectForbidden(commandID string) error {
	if IsWrite(commandID) {
		return fmt.Errorf("perch v1 is read-only: refusing %q", commandID)
	}
	if _, ok := createCommands[commandID]; ok {
		return fmt.Errorf("perch v1 is read-only: refusing %q (may create files)", commandID)
	}
	return nil
}
