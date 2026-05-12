package mastery_test

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

func TestLevelFor(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{0.00, mastery.LevelNovice},
		{0.39, mastery.LevelNovice},
		{0.40, mastery.LevelFamiliar},
		{0.64, mastery.LevelFamiliar},
		{0.65, mastery.LevelProficient},
		{0.84, mastery.LevelProficient},
		{0.85, mastery.LevelMastered},
		{1.00, mastery.LevelMastered},
		{1.50, mastery.LevelMastered}, // clamp-via-default
		{-0.10, mastery.LevelNovice},  // clamp-via-default
	}
	for _, tc := range cases {
		if got := mastery.LevelFor(tc.value); got != tc.want {
			t.Errorf("LevelFor(%.2f) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestLevelOrdinal(t *testing.T) {
	cases := []struct {
		level string
		want  int
	}{
		{mastery.LevelNovice, 0},
		{mastery.LevelFamiliar, 1},
		{mastery.LevelProficient, 2},
		{mastery.LevelMastered, 3},
		{"unknown", -1},
		{"", -1},
	}
	for _, tc := range cases {
		if got := mastery.LevelOrdinal(tc.level); got != tc.want {
			t.Errorf("LevelOrdinal(%q) = %d, want %d", tc.level, got, tc.want)
		}
	}
}
