package schemagen

import (
	"strings"
	"testing"
)

func TestRenderCreateTable_BigSerial(t *testing.T) {
	// AutoMigrate writes id columns as `bigint NOT NULL DEFAULT nextval(...)`;
	// the renderer should collapse that back to `bigserial` so output matches
	// the convention used in migration 000001.
	tbl := &Table{
		Name: "things",
		Columns: []*Column{
			{
				Name:     "id",
				DataType: "bigint",
				Nullable: false,
				Default:  strPtr("nextval('things_id_seq'::regclass)"),
			},
			{Name: "label", DataType: "text", Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
	got := RenderCreateTable(tbl)

	if strings.Contains(got, "nextval") {
		t.Errorf("expected nextval to be sugared away, got:\n%s", got)
	}
	if !strings.Contains(got, "id bigserial") {
		t.Errorf("expected `id bigserial`, got:\n%s", got)
	}
	if strings.Contains(got, "id bigserial NOT NULL") {
		t.Errorf("bigserial already implies NOT NULL, got:\n%s", got)
	}
}

func TestRenderAddColumn(t *testing.T) {
	cases := []struct {
		name string
		col  *Column
		want string
	}{
		{
			name: "nullable text",
			col:  &Column{Name: "display_name", DataType: "text", Nullable: true},
			want: "ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name text;",
		},
		{
			name: "not-null with default",
			col: &Column{
				Name:     "max_upload_size_mb",
				DataType: "bigint",
				Nullable: false,
				Default:  strPtr("500"),
			},
			want: "ALTER TABLE accounts ADD COLUMN IF NOT EXISTS max_upload_size_mb bigint NOT NULL DEFAULT 500;",
		},
		{
			name: "varchar with length",
			col: &Column{
				Name:      "code",
				DataType:  "character varying",
				MaxLength: int64Ptr(64),
				Nullable:  true,
			},
			want: "ALTER TABLE access_tokens ADD COLUMN IF NOT EXISTS code varchar(64);",
		},
	}
	tables := map[string]string{
		"nullable text":         "users",
		"not-null with default": "accounts",
		"varchar with length":   "access_tokens",
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderAddColumn(tables[tc.name], tc.col)
			if got != tc.want {
				t.Errorf("want %q got %q", tc.want, got)
			}
		})
	}
}

func TestRenderCreateTable_PreservesOtherDefaults(t *testing.T) {
	tbl := &Table{
		Name: "things",
		Columns: []*Column{
			{Name: "score", DataType: "numeric", Nullable: false, Default: strPtr("0")},
			{Name: "state", DataType: "text", Nullable: true, Default: strPtr("'active'::text")},
		},
	}
	got := RenderCreateTable(tbl)
	if !strings.Contains(got, "DEFAULT 0") {
		t.Errorf("numeric default lost:\n%s", got)
	}
	if !strings.Contains(got, "DEFAULT 'active'::text") {
		t.Errorf("text default lost:\n%s", got)
	}
}
