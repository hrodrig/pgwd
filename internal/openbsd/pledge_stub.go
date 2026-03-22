//go:build !openbsd

package openbsd

// ApplyPledge is a no-op on non-OpenBSD systems.
func ApplyPledge() {}
