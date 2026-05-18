package models_test

// This test enforces the "GORM acronym fields need explicit gorm:\"column:...\"
// tag" convention documented in CLAUDE.md ("Phase 7 patterns" /
// "Load-bearing model patterns") and surfaced repeatedly in the
// 2026-05-15 production audit. GORM's default NamingStrategy splits
// runs of uppercase letters incorrectly (e.g., `IDPEntityID` →
// `id_p_entity_id`, `LDAPHost` → `l_d_a_p_host`, `TOTPLastUsedWindow`
// → `t_o_t_p_last_used_window`). The SQL migration chain uses the
// canonical column names (`idp_entity_id`, `ldap_host`,
// `totp_last_used_window`, etc.), so any field whose Go name starts
// with two or more uppercase letters MUST carry an explicit
// `gorm:"column:..."` tag pinning the column name. The tag is the
// type-system enforcement of the convention; without it, the next
// AutoMigrate adds a duplicate column with the GORM-derived name and
// the parity test breaks.
//
// The reflect walker visits every exported field on every persisted
// model registered in models.AllModels() (which mirrors the
// AutoMigrate set in internal/db/postgres.go::allAutoMigrateModels()
// plus a handful of migration-only models). Embedded structs are
// recursed into so association fields don't hide acronym violations.

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// acronymPrefix matches a Go field name that begins with two or more
// uppercase ASCII letters — the trigger condition for GORM's incorrect
// snake_case split. `ID` alone matches (GORM autogen's "id" special-case
// still means we want an explicit `column:id` tag so the rule is
// uniformly enforceable). `URLPath` matches. `Url` does NOT match.
var acronymPrefix = regexp.MustCompile(`^[A-Z]{2,}`)

func TestGORMAcronymFieldsHaveColumnTag(t *testing.T) {
	var violations []string
	seen := map[reflect.Type]bool{}
	for _, m := range models.AllModels() {
		walk(reflect.TypeOf(m).Elem(), seen, &violations)
	}
	if len(violations) > 0 {
		t.Errorf(
			"found %d acronym-prefixed GORM field(s) without an explicit `gorm:\"column:...\"` tag.\n"+
				"GORM's default NamingStrategy will derive a wrong column name (e.g., `IDPEntityID` → `id_p_entity_id`)\n"+
				"and `AutoMigrate` will add a duplicate column the SQL chain doesn't expect, breaking TestSchemaParity_Wave1.\n"+
				"Fix by adding `gorm:\"column:<canonical_name>\"` — see CLAUDE.md \"GORM acronym column tags\" and\n"+
				"internal/domain/models/authentication_provider.go for the OIDC* example. Violations:\n  %s",
			len(violations),
			strings.Join(violations, "\n  "),
		)
	}
}

func walk(t reflect.Type, seen map[reflect.Type]bool, violations *[]string) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	if seen[t] {
		return
	}
	seen[t] = true

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		// Embedded structs (anonymous fields) — recurse so embedded
		// association columns don't slip through.
		if f.Anonymous {
			walk(f.Type, seen, violations)
			continue
		}
		if !acronymPrefix.MatchString(f.Name) {
			continue
		}
		gormTag := f.Tag.Get("gorm")
		if gormTag == "-" {
			// Explicit GORM-ignore. Field is not persisted.
			continue
		}
		// Association fields (foreignKey:...) declare relationships, not
		// columns — they don't generate a column on this table. Same for
		// many2many. They still need to round-trip through GORM's naming,
		// but no SQL column is created for them.
		lower := strings.ToLower(gormTag)
		if strings.Contains(lower, "foreignkey:") || strings.Contains(lower, "many2many:") || strings.Contains(lower, "polymorphic:") {
			continue
		}
		if hasColumnTag(gormTag) {
			continue
		}
		*violations = append(*violations,
			t.Name()+"."+f.Name+": GORM acronym fields need explicit column tag. "+
				"See CLAUDE.md \"GORM acronym column tags\".",
		)
	}
}

// hasColumnTag reports whether the gorm struct tag includes a `column:`
// directive. Other directives may surround it (`primaryKey`,
// `uniqueIndex`, `type:text`, etc.) — order is not significant.
func hasColumnTag(gormTag string) bool {
	for _, part := range strings.Split(gormTag, ";") {
		if strings.HasPrefix(strings.TrimSpace(part), "column:") {
			return true
		}
	}
	return false
}
