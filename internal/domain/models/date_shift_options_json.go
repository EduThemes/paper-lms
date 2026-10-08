package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// UnmarshalJSON accepts the date fields of DateShiftOptions either as
// RFC 3339 timestamps (what Paper writes back) or as plain calendar dates
// ("2026-01-12", what Canvas clients and date pickers send). A plain date
// is read as midnight UTC; the shifter only uses its calendar date.
func (o *DateShiftOptions) UnmarshalJSON(b []byte) error {
	type plain DateShiftOptions
	var raw struct {
		plain
		OldStartDate json.RawMessage `json:"old_start_date,omitempty"`
		NewStartDate json.RawMessage `json:"new_start_date,omitempty"`
		OldEndDate   json.RawMessage `json:"old_end_date,omitempty"`
		NewEndDate   json.RawMessage `json:"new_end_date,omitempty"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := DateShiftOptions(raw.plain)
	var err error
	if out.OldStartDate, err = parseShiftDate(raw.OldStartDate); err != nil {
		return fmt.Errorf("old_start_date: %w", err)
	}
	if out.NewStartDate, err = parseShiftDate(raw.NewStartDate); err != nil {
		return fmt.Errorf("new_start_date: %w", err)
	}
	if out.OldEndDate, err = parseShiftDate(raw.OldEndDate); err != nil {
		return fmt.Errorf("old_end_date: %w", err)
	}
	if out.NewEndDate, err = parseShiftDate(raw.NewEndDate); err != nil {
		return fmt.Errorf("new_end_date: %w", err)
	}
	*o = out
	return nil
}

func parseShiftDate(raw json.RawMessage) (*time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unrecognized date %q", s)
}
