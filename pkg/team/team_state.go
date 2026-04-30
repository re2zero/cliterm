// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package team

import "fmt"

func ValidateWorkerTransition(from, to string) error {
	validTargets, ok := ValidWorkerTransitions[from]
	if !ok {
		return fmt.Errorf("unknown worker status: %q", from)
	}
	for _, target := range validTargets {
		if target == to {
			return nil
		}
	}
	return fmt.Errorf("invalid worker transition: %q -> %q (allowed: %v)", from, to, validTargets)
}

func ValidateTaskTransition(from, to string) error {
	validTargets, ok := ValidTaskTransitions[from]
	if !ok {
		return fmt.Errorf("unknown task status: %q", from)
	}
	for _, target := range validTargets {
		if target == to {
			return nil
		}
	}
	return fmt.Errorf("invalid task transition: %q -> %q (allowed: %v)", from, to, validTargets)
}
