import { useEffect, useState, useRef } from 'react';
import { api } from '../services/api';

// Process-wide cache. The vocabulary payload is static per server build
// — no per-tenant variation in W2-E.1 — so re-fetching on every editor
// mount is wasteful. Cached forever within a page lifetime; the rare
// case where a deploy ships a new catalog while the user has the
// editor open is acceptable (worst case: a kind the UI knows about
// isn't yet decoded by the backend, and the server returns 400).
let cached = null;
let inflight = null;

// Resets the cache. Test-only — exported so vitest can isolate tests
// that need a fresh fetch.
export function _resetVocabularyCache() {
  cached = null;
  inflight = null;
}

// useGamificationVocabulary lazily fetches `/api/v1/gamification/vocabulary`
// (W2-E.1) and exposes it to the recipe-builder components. The
// returned `vocab` carries the catalog shape declared in
// `internal/service/gamification/vocabulary.go`:
//
//   {
//     triggers:      [{ kind, params: [{name,type,required,enum,ref,min,max,description}] }, ...],
//     predicates:    [...same shape],
//     effects:       [...same shape],
//     set_ops:       ["AND","OR","N_OF_M"],
//     audiences:     ["k5","m68","h912","higher_ed","corp","pro"],
//     scopes:        [...],
//     windows:       ["day","week","lifetime"],
//     mastery_levels:["novice","familiar","proficient","mastered"],
//   }
//
// `loading` is true on the initial fetch only — cached return is
// synchronous after the first mount within a page lifetime. `error`
// surfaces a failed fetch so the editor can render an inline alert
// rather than the silently-empty composer it would otherwise show.
export function useGamificationVocabulary() {
  const [vocab, setVocab] = useState(cached);
  const [loading, setLoading] = useState(!cached);
  const [error, setError] = useState(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    if (cached) return undefined;

    // Coalesce concurrent first-mount fetches across multiple editors
    // opened in the same tick (unlikely in practice, but cheap).
    if (!inflight) {
      inflight = api.gamification.getVocabulary().then(
        (data) => {
          cached = data;
          inflight = null;
          return data;
        },
        (err) => {
          inflight = null;
          throw err;
        },
      );
    }

    inflight
      .then((data) => {
        if (!mounted.current) return;
        setVocab(data);
        setLoading(false);
      })
      .catch((err) => {
        if (!mounted.current) return;
        setError(err);
        setLoading(false);
      });

    return () => {
      mounted.current = false;
    };
  }, []);

  return { vocab, loading, error };
}

// Helper: find a kind's param spec list. Returns [] when the catalog
// doesn't know the kind (recipe loaded from an older catalog version,
// etc.) — caller renders the raw JSON in that case.
export function paramsForKind(catalog, kind) {
  if (!Array.isArray(catalog)) return [];
  const spec = catalog.find((k) => k.kind === kind);
  return spec ? spec.params : [];
}
