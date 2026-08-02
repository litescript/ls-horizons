// SPDX-License-Identifier: Apache-2.0
// Copyright 2025-2026 Peter Brown (litescript.net)

package missions

import "strings"

// ResolveProfile finds a mission profile matching the given spacecraft name or code.
func ResolveProfile(nameOrCode string) *MissionProfile {
	normalized := strings.ToUpper(strings.TrimSpace(nameOrCode))
	if normalized == "" {
		return nil
	}
	for _, profile := range catalog {
		for _, alias := range profile.Aliases {
			if strings.ToUpper(alias) == normalized {
				return profile
			}
		}
	}
	return nil
}

// HasSpotlight returns true if the given spacecraft name/code has a mission profile.
func HasSpotlight(nameOrCode string) bool {
	return ResolveProfile(nameOrCode) != nil
}
