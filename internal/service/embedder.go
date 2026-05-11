package service

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// Embedder turns free text into a fixed-length dense vector.
//
// TODO(phase5+): swap HashingEmbedder for a real semantic embedder
// (OpenAI text-embedding-3-small @ 1536d, Cohere embed-multilingual-v3,
// or a local MiniLM via ONNX). Keep the same interface and dimension —
// just change the wiring in cmd/server/main.go and re-run a full
// re-index. The schema is dimension-pinned (vector(384)) so a different
// dimension also requires a migration to ALTER COLUMN TYPE.
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dim() int
}

// HashingEmbedder is a deterministic, dependency-free embedder using the
// "hashing trick": tokenize the input, hash each token to a bucket
// modulo Dim, and increment that bucket. The resulting vector is then
// L2-normalized so cosine similarity behaves like normalized dot product.
//
// This is NOT a semantic embedder — synonyms hash to different buckets —
// but it gives us bag-of-words ranking with zero external dependencies
// and lets every other layer (storage, ANN, API, UI) be production-quality
// and ready for a drop-in semantic embedder later.
type HashingEmbedder struct {
	D int
}

// NewHashingEmbedder constructs a HashingEmbedder. dim must be > 0;
// callers should pass models.EmbeddingDim (384) for schema compatibility.
func NewHashingEmbedder(dim int) *HashingEmbedder {
	if dim <= 0 {
		dim = 384
	}
	return &HashingEmbedder{D: dim}
}

func (h *HashingEmbedder) Dim() int { return h.D }

func (h *HashingEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, h.D)
	if text == "" {
		return vec, nil
	}
	for _, tok := range tokenize(text) {
		// Two independent hashes: one picks the bucket, the other picks
		// the sign. This keeps unrelated tokens from always reinforcing
		// the same axis (a common weakness of naive hashing trick impls).
		bucket := fnvHash32(tok) % uint32(h.D)
		sign := float32(1)
		if fnvHash32("sign:"+tok)%2 == 1 {
			sign = -1
		}
		vec[bucket] += sign
	}
	// L2-normalize so cosine similarity is comparable across documents
	// of different lengths.
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum > 0 {
		inv := float32(1.0 / math.Sqrt(sum))
		for i := range vec {
			vec[i] *= inv
		}
	}
	return vec, nil
}

// tokenize lowercases and splits on non-letter/digit runes, dropping tokens shorter than 2 chars.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) >= 2 && !isStopword(f) {
			out = append(out, f)
		}
	}
	return out
}

// Tiny English stoplist — enough to keep the toy embedder from being
// dominated by function words. Production embedders would use a real
// tokenizer + tokenizer model and skip this entirely.
var stopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "with": {}, "you": {}, "are": {},
	"this": {}, "that": {}, "have": {}, "but": {}, "not": {}, "from": {},
	"your": {}, "was": {}, "all": {}, "any": {}, "can": {}, "has": {},
	"will": {}, "they": {}, "their": {}, "our": {},
}

func isStopword(t string) bool { _, ok := stopwords[t]; return ok }

func fnvHash32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
