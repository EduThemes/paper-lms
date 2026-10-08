package qti

// Canvas calculated_question (Paper "formula") import for the standalone
// QTI importer. Canvas keeps the formula, the variable ranges and a list
// of precomputed (variables -> answer) sets in
// <itemproc_extension><calculated>; its resprocessing is just
// "<other/> scores", so without this block the item has no key.
//
// The output matches the formula config read by
// internal/service/quiz_formula.go: each attempt draws one answer set.

import (
	"encoding/json"
	"strconv"
	"strings"
)

type classicCalculated struct {
	AnswerTolerance string `xml:"answer_tolerance"`
	Formulas        struct {
		DecimalPlaces string   `xml:"decimal_places,attr"`
		Formula       []string `xml:"formula"`
	} `xml:"formulas"`
	Vars []struct {
		Name  string `xml:"name,attr"`
		Scale string `xml:"scale,attr"`
		Min   string `xml:"min"`
		Max   string `xml:"max"`
	} `xml:"vars>var"`
	VarSets []struct {
		Vars []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:",chardata"`
		} `xml:"var"`
		Answer string `xml:"answer"`
	} `xml:"var_sets>var_set"`
}

type formulaVar struct {
	Name      string  `json:"name"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Precision int     `json:"precision"`
}

type formulaVarValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type formulaAnswerSet struct {
	Variables []formulaVarValue `json:"variables"`
	Answer    float64           `json:"answer"`
}

type formulaConfig struct {
	ID              string             `json:"id"`
	Weight          float64            `json:"weight"`
	Formula         string             `json:"formula"`
	Variables       []formulaVar       `json:"variables"`
	Tolerance       string             `json:"tolerance,omitempty"`
	AnswerPrecision *int               `json:"answer_precision,omitempty"`
	AnswerSets      []formulaAnswerSet `json:"answer_sets,omitempty"`
}

// buildFormulaAnswers turns a calculated block into the formula config.
// ok=false when the item carries no calculated block.
func buildFormulaAnswers(item classicItem) (string, bool) {
	calc := item.Calculated
	if calc == nil {
		return "", false
	}
	cfg := formulaConfig{ID: item.Ident, Weight: 100, Tolerance: strings.TrimSpace(calc.AnswerTolerance)}
	if cfg.ID == "" {
		cfg.ID = "formula"
	}
	if n := len(calc.Formulas.Formula); n > 0 {
		cfg.Formula = strings.TrimSpace(calc.Formulas.Formula[n-1])
	}
	if dp, err := strconv.Atoi(strings.TrimSpace(calc.Formulas.DecimalPlaces)); err == nil && dp >= 0 {
		cfg.AnswerPrecision = &dp
	}
	for _, v := range calc.Vars {
		fv := formulaVar{Name: strings.TrimSpace(v.Name)}
		fv.Min, _ = strconv.ParseFloat(strings.TrimSpace(v.Min), 64)
		fv.Max, _ = strconv.ParseFloat(strings.TrimSpace(v.Max), 64)
		fv.Precision, _ = strconv.Atoi(strings.TrimSpace(v.Scale))
		if fv.Name != "" {
			cfg.Variables = append(cfg.Variables, fv)
		}
	}
	for _, set := range calc.VarSets {
		ans, err := strconv.ParseFloat(strings.TrimSpace(set.Answer), 64)
		if err != nil {
			continue
		}
		fs := formulaAnswerSet{Answer: ans}
		for _, v := range set.Vars {
			if val, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64); err == nil {
				fs.Variables = append(fs.Variables, formulaVarValue{Name: strings.TrimSpace(v.Name), Value: val})
			}
		}
		cfg.AnswerSets = append(cfg.AnswerSets, fs)
	}
	if cfg.Formula == "" && len(cfg.AnswerSets) == 0 {
		return "", false
	}
	b, err := json.Marshal([]formulaConfig{cfg})
	if err != nil {
		return "", false
	}
	return string(b), true
}
