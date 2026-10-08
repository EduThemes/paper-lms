package service_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// migrationStepsAbove counts the migrations numbered above version. The
// chain is not contiguous (parallel work reserves number blocks), so
// "latest - version" is not the number of steps to roll back.
func migrationStepsAbove(t *testing.T, version uint) int {
	t.Helper()
	entries, err := os.ReadDir("../db/migrations")
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		num, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseUint(num, 10, 64)
		require.NoError(t, err, name)
		if uint(v) > version {
			n++
		}
	}
	return n
}
