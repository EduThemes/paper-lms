package effects

import (
	"encoding/json"
	"strings"
	"testing"
)

// makeRaw wraps an AwardCurrency struct literal into the JSON shape the
// factory expects: the struct's own fields plus a "kind" discriminator.
// We do this by marshaling the struct and then patching "kind" in via a
// generic map so we don't depend on a JSON tag layout in the source file.
func makeRaw(t *testing.T, kind string, payload any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	m["kind"] = kind
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	return out
}

func ptrFloat(v float64) *float64 { return &v }

func TestDecodeEffect(t *testing.T) {
	t.Run("single AwardCurrency round-trips", func(t *testing.T) {
		raw := makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 10})
		eff, err := DecodeEffect(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ac, ok := eff.(AwardCurrency)
		if !ok {
			t.Fatalf("expected AwardCurrency, got %T", eff)
		}
		if ac.Code != "xp" {
			t.Errorf("Code = %q, want %q", ac.Code, "xp")
		}
		if ac.Amount != 10 {
			t.Errorf("Amount = %d, want 10", ac.Amount)
		}
		if ac.Multiplier != nil {
			t.Errorf("Multiplier = %v, want nil", *ac.Multiplier)
		}
		if eff.Kind() != "AwardCurrency" {
			t.Errorf("Kind() = %q, want AwardCurrency", eff.Kind())
		}
	})

	t.Run("AwardCurrency with Multiplier round-trips", func(t *testing.T) {
		raw := makeRaw(t, "AwardCurrency", AwardCurrency{
			Code:       "xp",
			Amount:     20,
			Multiplier: ptrFloat(1.5),
		})
		eff, err := DecodeEffect(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ac := eff.(AwardCurrency)
		if ac.Multiplier == nil {
			t.Fatalf("Multiplier is nil, expected 1.5")
		}
		if *ac.Multiplier != 1.5 {
			t.Errorf("*Multiplier = %v, want 1.5", *ac.Multiplier)
		}
	})

	t.Run("unknown kind returns error", func(t *testing.T) {
		// AwardBadge moved from "unknown" to "registered" in W2-D; use a
		// truly-unknown discriminator here (e.g. ReleaseContent will land
		// in W2-E, BranchPath later still).
		raw := json.RawMessage(`{"kind":"ReleaseContent","item_id":42}`)
		_, err := DecodeEffect(raw)
		if err == nil {
			t.Fatalf("expected error for unknown kind")
		}
		if !strings.Contains(err.Error(), "unknown effect kind") {
			t.Errorf("error %q does not mention 'unknown effect kind'", err.Error())
		}
		if !strings.Contains(err.Error(), "ReleaseContent") {
			t.Errorf("error %q does not mention the offending kind", err.Error())
		}
	})

	t.Run("AwardCurrency with Amount=0 returns error", func(t *testing.T) {
		raw := makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 0})
		_, err := DecodeEffect(raw)
		if err == nil {
			t.Fatalf("expected error for Amount=0")
		}
		if !strings.Contains(err.Error(), "Amount") {
			t.Errorf("error %q does not mention Amount", err.Error())
		}
	})

	t.Run("AwardCurrency with negative Amount returns error", func(t *testing.T) {
		raw := makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: -5})
		_, err := DecodeEffect(raw)
		if err == nil {
			t.Fatalf("expected error for negative Amount")
		}
	})

	t.Run("AwardCurrency with empty Code returns error", func(t *testing.T) {
		raw := makeRaw(t, "AwardCurrency", AwardCurrency{Code: "", Amount: 10})
		_, err := DecodeEffect(raw)
		if err == nil {
			t.Fatalf("expected error for empty Code")
		}
		if !strings.Contains(err.Error(), "Code") {
			t.Errorf("error %q does not mention Code", err.Error())
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		raw := json.RawMessage(`{not json`)
		_, err := DecodeEffect(raw)
		if err == nil {
			t.Fatalf("expected error for malformed JSON")
		}
	})
}

func TestDecodeEffects(t *testing.T) {
	t.Run("array of multiple AwardCurrency decodes in order", func(t *testing.T) {
		items := []json.RawMessage{
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 10}),
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "coins", Amount: 5}),
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 25, Multiplier: ptrFloat(2.0)}),
		}
		arr, err := json.Marshal(items)
		if err != nil {
			t.Fatalf("marshal array: %v", err)
		}

		got, err := DecodeEffects(arr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}

		wantCodes := []string{"xp", "coins", "xp"}
		wantAmounts := []int64{10, 5, 25}
		for i, eff := range got {
			ac, ok := eff.(AwardCurrency)
			if !ok {
				t.Fatalf("[%d]: expected AwardCurrency, got %T", i, eff)
			}
			if ac.Code != wantCodes[i] {
				t.Errorf("[%d]: Code = %q, want %q", i, ac.Code, wantCodes[i])
			}
			if ac.Amount != wantAmounts[i] {
				t.Errorf("[%d]: Amount = %d, want %d", i, ac.Amount, wantAmounts[i])
			}
		}
		// Last one had a multiplier; verify it survived.
		if got[2].(AwardCurrency).Multiplier == nil || *got[2].(AwardCurrency).Multiplier != 2.0 {
			t.Errorf("third effect's Multiplier did not round-trip")
		}
	})

	t.Run("array with one bad entry surfaces the index", func(t *testing.T) {
		items := []json.RawMessage{
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 10}),
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 0}), // bad
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 7}),
		}
		arr, err := json.Marshal(items)
		if err != nil {
			t.Fatalf("marshal array: %v", err)
		}

		_, err = DecodeEffects(arr)
		if err == nil {
			t.Fatalf("expected error from bad entry at index 1")
		}
		if !strings.Contains(err.Error(), "effect 1") {
			t.Errorf("error %q does not mention index 1", err.Error())
		}
	})

	t.Run("array with unknown kind surfaces the index", func(t *testing.T) {
		items := []json.RawMessage{
			makeRaw(t, "AwardCurrency", AwardCurrency{Code: "xp", Amount: 10}),
			json.RawMessage(`{"kind":"Notify","message":"hi"}`),
		}
		arr, err := json.Marshal(items)
		if err != nil {
			t.Fatalf("marshal array: %v", err)
		}

		_, err = DecodeEffects(arr)
		if err == nil {
			t.Fatalf("expected error from unknown kind at index 1")
		}
		if !strings.Contains(err.Error(), "effect 1") {
			t.Errorf("error %q does not mention index 1", err.Error())
		}
		if !strings.Contains(err.Error(), "unknown effect kind") {
			t.Errorf("error %q does not mention 'unknown effect kind'", err.Error())
		}
	})

	t.Run("empty array decodes to empty slice", func(t *testing.T) {
		got, err := DecodeEffects(json.RawMessage(`[]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("non-array input returns error", func(t *testing.T) {
		_, err := DecodeEffects(json.RawMessage(`{"kind":"AwardCurrency"}`))
		if err == nil {
			t.Fatalf("expected error for non-array input")
		}
	})
}
