package service

import (
	"strings"
	"testing"
)

// Outgoing email bodies use the conservative email policy: a positioned
// overlay (fine inside Paper's contained viewers) loses its positioning.
func TestWrapHTMLTemplate_UsesEmailPolicy(t *testing.T) {
	body := `<div style="position:absolute;top:0;left:0;z-index:100;transform:scale(3);color:#c00">Overlay</div>`
	out, err := (&NotificationDeliveryService{}).wrapHTMLTemplate("Notice", body)
	if err != nil {
		t.Fatalf("wrapHTMLTemplate: %v", err)
	}
	i := strings.Index(out, "Overlay")
	if i < 0 {
		t.Fatalf("body text missing:\n%s", out)
	}
	tag := out[strings.LastIndex(out[:i], "<div"):i]
	for _, bad := range []string{"position", "top:", "left:", "z-index", "transform"} {
		if strings.Contains(tag, bad) {
			t.Errorf("email body keeps %q: %s", bad, tag)
		}
	}
	if !strings.Contains(tag, "color: #c00") {
		t.Errorf("email body lost its colour: %s", tag)
	}
}
