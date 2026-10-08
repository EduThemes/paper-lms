package handlers

import "testing"

func TestSanitizeReturnTo(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"", "/", true},
		{"/courses/3", "/courses/3", true},
		{"/courses/3?tab=x#frag", "/courses/3?tab=x#frag", true},
		{"/x?session_token=leak&a=1", "/x?a=1", true},
		{"https://api.example.com/dashboard", "/dashboard", true},
		{"https://lms.example.com/calendar", "/calendar", true},
		{"https://evil.example.net/", "", false},
		{"//evil.example.net/", "", false},
		{"javascript:alert(1)", "", false},
		{"courses/3", "", false},
		{"/\\evil.example.net", "", false},
	}
	for _, tc := range cases {
		got, ok := sanitizeReturnTo(tc.in, "api.example.com", "https://lms.example.com")
		if ok != tc.ok || got != tc.want {
			t.Errorf("sanitizeReturnTo(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
