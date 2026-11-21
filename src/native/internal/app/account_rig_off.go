//go:build !automation

package app

import "github.com/pablopunk/nvm/native/internal/backend/auth"

// applyRigBackend leaves the account wiring as it is. Only automation builds have the rig's fake backend override.
func applyRigBackend(*auth.Deps) {}
