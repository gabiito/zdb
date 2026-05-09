package core

import (
	"crypto/rand"
	"encoding/hex"
)

// newReqID generates a short random request ID for tracking in-flight async ops.
func newReqID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Async cmd builders are defined in app.go once the App type is declared.
// This file holds the shared newReqID helper and any package-level cmd utilities.
