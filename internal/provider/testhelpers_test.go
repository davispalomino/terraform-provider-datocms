// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import "testing"

// mustAssert asserts that v is of type T, failing the test otherwise.
func mustAssert[T any](t *testing.T, v any) T {
	t.Helper()
	typed, ok := v.(T)
	if !ok {
		t.Fatalf("expected %T, got %T", *new(T), v)
	}
	return typed
}
