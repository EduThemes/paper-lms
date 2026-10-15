package models

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
)

// NullJSON is a nullable jsonb column holding a raw JSON document.
//
// datatypes.JSON scans SQL NULL into the literal `null` and writes it back
// as the jsonb value 'null', so "no value" does not survive a load+save.
// NullJSON keeps NULL as NULL: a NULL column scans to nil, and nil (or a
// JSON null) is written as SQL NULL. It marshals to the raw document, or
// to `null` when empty.
type NullJSON []byte

// IsNull reports whether the value is SQL NULL / JSON null.
func (j NullJSON) IsNull() bool {
	t := bytes.TrimSpace(j)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

// Value implements driver.Valuer.
func (j NullJSON) Value() (driver.Value, error) {
	if j.IsNull() {
		return nil, nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner.
func (j *NullJSON) Scan(value interface{}) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append(NullJSON(nil), v...)
	case string:
		*j = NullJSON(v)
	default:
		return errors.New(fmt.Sprint("NullJSON: cannot scan ", value))
	}
	if j.IsNull() {
		*j = nil
	}
	return nil
}

// MarshalJSON writes the raw document, or null.
func (j NullJSON) MarshalJSON() ([]byte, error) {
	if j.IsNull() {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON keeps the raw document; JSON null becomes nil.
func (j *NullJSON) UnmarshalJSON(b []byte) error {
	if NullJSON(b).IsNull() {
		*j = nil
		return nil
	}
	*j = append(NullJSON(nil), b...)
	return nil
}

// GormDataType is the column type AutoMigrate creates.
func (NullJSON) GormDataType() string {
	return "jsonb"
}
