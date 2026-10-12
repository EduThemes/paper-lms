package service

// Student-facing redaction of quiz question answer keys.
//
// quiz_questions.answers holds the full answer key (weights, accepted
// short-answer strings, numerical values, matching right_ids, hot-spot
// regions, categorization bucket assignments, …). Anyone who is not course
// staff must only ever see the parts of that JSON needed to RENDER the item:
// option ids and labels. StudentSafeQuizAnswers produces that view per item
// type; unknown types are redacted to "[]" (default-deny).

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	mathrand "math/rand"
	"strconv"
	"strings"
	"sync"
)

type safeChoice struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	BlankID string `json:"blank_id,omitempty"`
}

type safeMatchPair struct {
	ID   string `json:"id"`
	Left string `json:"left"`
	Text string `json:"text"`
}

type safeLabel struct {
	ID    string `json:"id"`
	Text  string `json:"text,omitempty"`
	Label string `json:"label,omitempty"`
}

// rawOption is a loose decode of one answers entry used only for
// redaction (texts and labels are kept as raw JSON-ish strings).
type rawOption struct {
	ID      string     `json:"id"`
	Text    flexString `json:"text"`
	Left    string     `json:"left"`
	Right   string     `json:"right"`
	RightID string     `json:"right_id"`
	MatchID string     `json:"match_id"`
	BlankID string     `json:"blank_id"`
	Weight  float64    `json:"weight"`
	Items   []struct {
		ID   string     `json:"id"`
		Text flexString `json:"text"`
	} `json:"items"`
	Buckets []struct {
		ID    string     `json:"id"`
		Label flexString `json:"label"`
	} `json:"buckets"`
	ImageURL string `json:"image_url"`
}

var (
	quizShuffleKeyMu sync.RWMutex
	quizShuffleKey   []byte
)

// SetQuizShuffleKey keys QuizShuffleSeed with a server secret. Without a
// secret the seed is a function of ids the student can see, and the code is
// public: a student could recompute the presentation order and undo it to
// read the authored (= correct) order of an ordering or matching item.
// Production sets it at startup from a secret every instance shares, so the
// order is the same whichever server answers.
func SetQuizShuffleKey(secret []byte) {
	quizShuffleKeyMu.Lock()
	defer quizShuffleKeyMu.Unlock()
	quizShuffleKey = append([]byte(nil), secret...)
}

func currentQuizShuffleKey() []byte {
	quizShuffleKeyMu.RLock()
	key := quizShuffleKey
	quizShuffleKeyMu.RUnlock()
	if len(key) > 0 {
		return key
	}
	// Not configured (tests, tools): a random per-process key is still
	// unpredictable; only cross-restart stability is lost.
	quizShuffleKeyMu.Lock()
	defer quizShuffleKeyMu.Unlock()
	if len(quizShuffleKey) == 0 {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			panic("quiz shuffle key: " + err.Error())
		}
		quizShuffleKey = k
	}
	return quizShuffleKey
}

// QuizShuffleSeed derives a stable shuffle seed from the given ids so the
// redacted order is stable across reloads of the same attempt. The seed is
// an HMAC under the server's shuffle key, so it can't be recomputed from
// the ids alone.
func QuizShuffleSeed(ids ...uint) int64 {
	mac := hmac.New(sha256.New, currentQuizShuffleKey())
	for _, id := range ids {
		_, _ = mac.Write([]byte(strconv.FormatUint(uint64(id), 10)))
		_, _ = mac.Write([]byte{':'})
	}
	return int64(binary.BigEndian.Uint64(mac.Sum(nil)[:8]) & 0x7fffffffffffffff)
}

// seededShuffle returns a uniformly shuffled copy of s for the given seed.
//
// The shuffle must be UNIFORM — including the authored order itself.
// Excluding (or remapping) the identity permutation leaks the key: a
// two-item ordering question would always come back reversed, and a
// two-pair matching pool would always be swapped, so the student could
// read the answer off the presentation. A uniform permutation carries no
// information about the authored (= correct) order.
func seededShuffle[T any](s []T, seed int64) []T {
	out := make([]T, len(s))
	copy(out, s)
	if len(out) < 2 {
		return out
	}
	// A seeded PRNG is intended: the order must be reproducible for the
	// same attempt. The seed itself is keyed (QuizShuffleSeed).
	r := mathrand.New(mathrand.NewSource(seed)) // #nosec G404 -- reproducible shuffle; the seed is HMAC-keyed
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// StudentSafeQuizAnswers returns the answers JSON with every answer-key
// field removed, shaped for the student item player. seed drives the
// deterministic shuffle of order-revealing lists (ordering items, matching
// right-hand pools).
func StudentSafeQuizAnswers(questionType, answers string, seed int64) string {
	qType := NormalizeQuizQuestionType(questionType)
	trimmed := strings.TrimSpace(answers)

	switch qType {
	case "multiple_choice", "true_false", "multiple_answer", "multiple_dropdown":
		var opts []rawOption
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil {
			return "[]"
		}
		out := make([]safeChoice, 0, len(opts))
		for _, o := range opts {
			c := safeChoice{ID: o.ID, Text: o.Text.String()}
			if qType == "multiple_dropdown" {
				c.BlankID = o.BlankID
			}
			out = append(out, c)
		}
		return mustJSON(out)

	case "ordering":
		var opts []rawOption
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil {
			return "[]"
		}
		items := make([]safeChoice, 0, len(opts))
		for _, o := range opts {
			if o.ID == "" || o.Weight < 0 {
				continue
			}
			items = append(items, safeChoice{ID: o.ID, Text: o.Text.String()})
		}
		return mustJSON(seededShuffle(items, seed))

	case "matching":
		var opts []rawOption
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil {
			return "[]"
		}
		pairs := []safeMatchPair{}
		pool := []safeChoice{}
		poolSeen := map[string]bool{}
		for _, o := range opts {
			if o.BlankID == matchOptionsSentinel {
				if o.ID != "" && !poolSeen[o.ID] {
					poolSeen[o.ID] = true
					pool = append(pool, safeChoice{ID: o.ID, Text: o.Text.String(), BlankID: matchOptionsSentinel})
				}
				continue
			}
			key := o.Left
			if key == "" {
				key = o.ID
			}
			display := o.Left
			if display == "" {
				display = o.Text.String()
			}
			pairs = append(pairs, safeMatchPair{ID: o.ID, Left: key, Text: display})
			right := o.RightID
			if right == "" {
				right = o.MatchID
			}
			if right != "" && !poolSeen[right] && o.Right != "" {
				poolSeen[right] = true
				pool = append(pool, safeChoice{ID: right, Text: o.Right, BlankID: matchOptionsSentinel})
			}
		}
		pool = seededShuffle(pool, seed)
		out := make([]interface{}, 0, len(pairs)+len(pool))
		for _, p := range pairs {
			out = append(out, p)
		}
		for _, p := range pool {
			out = append(out, p)
		}
		return mustJSON(out)

	case "fill_in_multiple_blanks":
		// Only the blank ids (no accepted strings).
		blanks := []map[string]string{}
		seen := map[string]bool{}
		if strings.HasPrefix(trimmed, "[") {
			var opts []rawOption
			if err := json.Unmarshal([]byte(trimmed), &opts); err != nil {
				return "[]"
			}
			for _, o := range opts {
				if o.BlankID != "" && !seen[o.BlankID] {
					seen[o.BlankID] = true
					blanks = append(blanks, map[string]string{"blank_id": o.BlankID})
				}
			}
		} else {
			var m map[string]json.RawMessage
			if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
				return "[]"
			}
			for k := range m {
				blanks = append(blanks, map[string]string{"blank_id": k})
			}
		}
		return mustJSON(blanks)

	case "categorization":
		var opts []rawOption
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil || len(opts) == 0 {
			return "[]"
		}
		cfg := opts[0]
		items := make([]safeLabel, 0, len(cfg.Items))
		for _, it := range cfg.Items {
			items = append(items, safeLabel{ID: it.ID, Text: it.Text.String()})
		}
		buckets := make([]safeLabel, 0, len(cfg.Buckets))
		for _, b := range cfg.Buckets {
			buckets = append(buckets, safeLabel{ID: b.ID, Label: b.Label.String()})
		}
		return mustJSON([]map[string]interface{}{{"id": cfg.ID, "items": items, "buckets": buckets}})

	case "hot_spot":
		var opts []rawOption
		if err := json.Unmarshal([]byte(trimmed), &opts); err != nil || len(opts) == 0 {
			return "[]"
		}
		return mustJSON([]map[string]interface{}{{"id": opts[0].ID, "image_url": opts[0].ImageURL}})
	}

	// short_answer, essay, numerical_question, fill_in_the_blank, formula,
	// file_upload, text_only, and anything unknown: nothing in answers is
	// needed to render the item, and for most of them answers IS the key.
	return "[]"
}
