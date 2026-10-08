package service_test

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// migrationStepsAbove counts the migrations whose version is above target.
// Migration numbers are not contiguous (feature lanes use reserved blocks),
// so "latest - target" is not the number of steps to roll back.
func migrationStepsAbove(t *testing.T, target uint) int {
	t.Helper()
	entries, err := os.ReadDir("../db/migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(prefix, 10, 64)
		if err == nil && uint(v) > target {
			n++
		}
	}
	return n
}
