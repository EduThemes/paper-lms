package service_test

// Regression tests for the Canvas-parity grading fixes (bug ids B026,
// B027, B028, B029, B035, B124, B130). They drive CompleteSubmission through
// the shared runNewTypesCase harness so the answer-key shapes are exactly
// what the editors / importers persist.

import "testing"

func TestParityGrading(t *testing.T) {
	cases := []newTypesCase{
		// ---- B026: matching cannot be shotgunned ----
		{
			name:         "matching_shotgun_every_right_per_left_no_full_credit",
			questionType: "matching",
			points:       4.0,
			answersJSON:  `[{"left":"a","right_id":"r1","weight":100},{"left":"b","right_id":"r2","weight":100}]`,
			submitted:    `[{"left":"a","right_id":"r2"},{"left":"a","right_id":"r1"},{"left":"b","right_id":"r1"},{"left":"b","right_id":"r2"}]`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "only the first pair per left counts",
		},
		{
			name:         "matching_canvas_import_shape_match_id_and_pool",
			questionType: "matching",
			points:       2.0,
			answersJSON:  `[{"id":"1","text":"France","match_id":"m1","weight":100},{"id":"2","text":"UK","match_id":"m2","weight":100},{"id":"m1","text":"Paris","blank_id":"_match_options_"},{"id":"m2","text":"London","blank_id":"_match_options_"},{"id":"m3","text":"Rome","blank_id":"_match_options_"}]`,
			submitted:    `[{"left":"1","right_id":"m1"},{"left":"2","right_id":"m3"}]`,
			wantScore:    1.0,
			wantWorkflow: "complete",
			note:         "match_id alias; pool entries are not pairs",
		},
		// ---- B027: categorization editor shape ----
		{
			name:         "categorization_editor_shape_all_correct",
			questionType: "categorization",
			points:       4.0,
			answersJSON:  `[{"id":"cfg","weight":100,"items":[{"id":"i1","text":"apple"},{"id":"i2","text":"carrot"}],"buckets":[{"id":"fruit","label":"Fruit","item_ids":["i1"]},{"id":"veg","label":"Veg","item_ids":["i2"]}]}]`,
			submitted:    `{"i1":"fruit","i2":"veg"}`,
			wantScore:    4.0,
			wantWorkflow: "complete",
		},
		{
			name:         "categorization_editor_shape_empty_submission_scores_zero",
			questionType: "categorization",
			points:       4.0,
			answersJSON:  `[{"id":"cfg","weight":100,"items":[{"id":"i1","text":"apple"}],"buckets":[{"id":"fruit","label":"Fruit","item_ids":["i1"]}]}]`,
			submitted:    `{}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "pre-fix: synthetic config key compared '' == '' → full credit",
		},
		{
			name:         "categorization_legacy_option_without_right_id_never_matches",
			questionType: "categorization",
			points:       4.0,
			answersJSON:  `[{"id":"a1","weight":100}]`,
			submitted:    `{}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		// ---- B028: ordering / hot spot / formula editor shapes ----
		{
			name:         "ordering_editor_shape_without_weights",
			questionType: "ordering",
			points:       3.0,
			answersJSON:  `[{"id":"o1","text":"First","position":0},{"id":"o2","text":"Second","position":1},{"id":"o3","text":"Third","position":2}]`,
			submitted:    `["o1","o2","o3"]`,
			wantScore:    3.0,
			wantWorkflow: "complete",
		},
		{
			name:         "ordering_editor_shape_partial",
			questionType: "ordering",
			points:       3.0,
			answersJSON:  `[{"id":"o1","text":"First"},{"id":"o2","text":"Second"},{"id":"o3","text":"Third"}]`,
			submitted:    `["o1","o3","o2"]`,
			wantScore:    1.0,
			wantWorkflow: "complete",
		},
		{
			name:         "hot_spot_editor_regions_inside",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h","weight":100,"image_url":"/x.png","regions":[{"id":"r1","x":0.2,"y":0.2,"w":0.3,"h":0.3}]}]`,
			submitted:    `{"x":0.3,"y":0.4}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "hot_spot_editor_regions_origin_click_is_wrong",
			questionType: "hot_spot",
			points:       2.0,
			answersJSON:  `[{"id":"h","weight":100,"image_url":"/x.png","regions":[{"id":"r1","x":0.2,"y":0.2,"w":0.3,"h":0.3}]}]`,
			submitted:    `{"x":0,"y":0}`,
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "pre-fix only (0,0) scored",
		},
		{
			name:         "formula_editor_answer_value_with_tolerance",
			questionType: "formula",
			points:       2.0,
			answersJSON:  `[{"id":"f","text":"","weight":100,"formula":"","variables":[],"tolerance":0.5,"answer_value":"10"}]`,
			submitted:    "10.4",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "formula_editor_blank_submission_is_wrong",
			questionType: "formula",
			points:       2.0,
			answersJSON:  `[{"id":"f","text":"","weight":100,"formula":"","variables":[],"tolerance":0,"answer_value":"10"}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
			note:         "pre-fix a blank answer matched the blank key",
		},
		{
			name:         "formula_variable_mode_without_value_needs_review",
			questionType: "formula",
			points:       2.0,
			answersJSON:  `[{"id":"f","text":"","weight":100,"formula":"x*2","variables":[{"name":"x","min":1,"max":5,"precision":0}],"tolerance":0,"answer_value":""}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "pending_review",
		},
		// ---- B029: added short-answer alternatives (weight 0) match ----
		{
			name:         "short_answer_added_alternative_weight_zero_matches",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"color","weight":100},{"id":"a2","text":"colour","weight":0}]`,
			submitted:    "Colour",
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "short_answer_blank_never_matches_blank_key",
			questionType: "short_answer",
			points:       2.0,
			answersJSON:  `[{"id":"a1","text":"","weight":0}]`,
			submitted:    "",
			wantScore:    0.0,
			wantWorkflow: "complete",
		},
		// ---- B035: legacy imported type names and Canvas FIMB array shape ----
		{
			name:         "legacy_multiple_answers_type_is_graded",
			questionType: "multiple_answers",
			points:       2.0,
			answersJSON:  `[{"id":"a","text":"A","weight":100},{"id":"b","text":"B","weight":100},{"id":"c","text":"C","weight":0}]`,
			submitted:    `["a","b"]`,
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "legacy_multiple_dropdowns_type_is_graded",
			questionType: "multiple_dropdowns",
			points:       2.0,
			answersJSON:  `[{"id":"o1","text":"red","weight":100,"blank_id":"color"},{"id":"o2","text":"blue","weight":0,"blank_id":"color"}]`,
			submitted:    `{"color":"o1"}`,
			wantScore:    2.0,
			wantWorkflow: "complete",
		},
		{
			name:         "fimb_canvas_array_shape",
			questionType: "fill_in_multiple_blanks",
			points:       2.0,
			answersJSON:  `[{"id":"x1","text":"red","blank_id":"color","weight":100},{"id":"x2","text":"crimson","blank_id":"color","weight":100},{"id":"x3","text":"Phoenix","blank_id":"city","weight":100}]`,
			submitted:    `{"color":"Crimson","city":"tucson"}`,
			wantScore:    1.0,
			wantWorkflow: "complete",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) { runNewTypesCase(t, tc) })
	}
}
