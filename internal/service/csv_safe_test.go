package service

import "testing"

func TestSanitizeCSVCell(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"equals formula", "=1+1", "'=1+1"},
		{"plus", "+1", "'+1"},
		{"minus", "-1", "'-1"},
		{"at", "@SUM(A1)", "'@SUM(A1)"},
		{"tab", "\tcmd", "'\tcmd"},
		{"carriage return", "\rcmd", "'\rcmd"},
		{"classic DDE payload", "=cmd|'/c calc'!A1", "'=cmd|'/c calc'!A1"},
		{"plain text untouched", "Algebra I", "Algebra I"},
		{"number untouched", "250", "250"},
		{"email untouched", "alice@example.com", "alice@example.com"},
		{"empty untouched", "", ""},
		{"leading space untouched", " =not a formula", " =not a formula"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeCSVCell(c.in); got != c.want {
				t.Errorf("sanitizeCSVCell(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSanitizeCSVRow(t *testing.T) {
	got := sanitizeCSVRow([]string{"=evil", "safe", "+danger"})
	want := []string{"'=evil", "safe", "'+danger"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
