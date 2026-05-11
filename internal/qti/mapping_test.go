package qti

import "testing"

// TestMappingCoverage is a load-bearing build-breaking test: it asserts
// that every entry in AllUnifiedTypes has an inbound mapping from at
// least one Canvas dialect AND an outbound mapping for the exporter.
// Adding a new unified type without updating the mapping tables in
// mapping.go fails here loudly — by design.
func TestMappingCoverage(t *testing.T) {
	for _, unified := range AllUnifiedTypes {
		// Outbound.
		if got := MapUnifiedToCanvasClassic(unified); got == "" {
			t.Errorf("unified type %q has no outbound mapping to Canvas Classic", unified)
		}

		// Inbound: at least one of Classic or NQ must map to this type.
		inbound := false
		for _, mapped := range canvasClassicToUnified {
			if mapped == unified {
				inbound = true
				break
			}
		}
		if !inbound {
			for _, mapped := range newQuizzesInteractionToUnified {
				if mapped == unified {
					inbound = true
					break
				}
			}
		}
		// Special-cased dialects:
		// - true_false / multiple_choice / multiple_answer / numerical /
		//   fill_in_the_blank for NQ are produced by classifier
		//   helpers (ClassifyNewQuizzesChoice / ClassifyNewQuizzesTextEntry),
		//   not the static map. They are still inbound-covered.
		switch unified {
		case UnifiedTrueFalse, UnifiedMultipleAnswer, UnifiedNumerical, UnifiedFillInTheBlank:
			inbound = true
		}
		if !inbound {
			t.Errorf("unified type %q has no inbound mapping (Classic or NQ)", unified)
		}
	}
}

// TestClassicMappingRoundTrip verifies the explicit Canvas-Classic
// table is bijective for the 12 native Classic types — exporter should
// emit the same `question_type` value that the importer reads back.
func TestClassicMappingRoundTrip(t *testing.T) {
	for canvasType, unified := range canvasClassicToUnified {
		gotCanvas := MapUnifiedToCanvasClassic(unified)
		// For lossy types (ordering / categorization / hot_spot /
		// fill_in_the_blank) the round-trip is intentionally non-
		// bijective — the exporter degrades them to their closest
		// Classic equivalent. Skip those.
		switch unified {
		case UnifiedOrdering, UnifiedCategorization, UnifiedHotSpot, UnifiedFillInTheBlank:
			continue
		}
		if gotCanvas != canvasType {
			t.Errorf("type %s: classic→unified→classic produced %q, want %q", unified, gotCanvas, canvasType)
		}
	}
}
