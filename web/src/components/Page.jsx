// <Page> — render-state wrapper for react-query-backed page components.
//
// Owns the 4-state JSX that 99 hand-rolled `useState(loading/error)
// + useEffect + try/catch` blocks were each re-implementing:
//
//   1. `query.isLoading`  → skeleton inside <Layout>
//   2. `query.isError`    → error card with "Try again" → query.refetch()
//   3. `empty(data)`      → empty-state message (callsite supplies the
//                            predicate; many list pages need it)
//   4. default            → call `children(data)` so the page renders
//                            against a non-null, non-error payload.
//
// Usage:
//   const query = useCourse(courseId);
//   return (
//     <Page query={query} title="Course">
//       {(course) => <CourseDetail course={course} />}
//     </Page>
//   );
//
// Props:
//   - `query` (required): the react-query `UseQueryResult`. We read
//     `isLoading`, `isError`, `error`, `data`, `refetch`.
//   - `title` (optional): rendered in loading/error/empty states so the
//     page outline stays visible during skeleton + error frames.
//   - `empty` (optional): `(data) => boolean`. When true, renders the
//     empty card instead of `children(data)`. Pass a predicate like
//     `(arr) => arr.length === 0` for list pages.
//   - `emptyMessage` (optional): override the default empty-state copy.
//   - `loadingFallback` (optional): override the default skeleton. Pass
//     a React node when the page wants a custom shimmer (e.g. a multi-
//     column dashboard).
//   - `children`: required render-prop receiving `data` (never null in
//     this branch — the loading and error states short-circuit first).
//
// Why a render-prop instead of `<Page>{...children}</Page>`? Because
// the caller needs typed access to the resolved data, and we don't
// want pages reading `query.data` directly (it can be `undefined`
// during the first render before isLoading flips — `children(data)`
// is only called once data is defined-and-fresh).

import React from 'react';
import Layout from './Layout';
import { Skeleton } from '@/components/ui/skeleton';

function Header({ title }) {
  if (!title) return null;
  return <h1 className="text-2xl font-bold text-text-primary mb-6">{title}</h1>;
}

function DefaultLoadingFallback() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-9 w-48" />
      <Skeleton className="h-12 w-full" />
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} className="h-16 w-full" />
      ))}
    </div>
  );
}

export default function Page({
  query,
  title,
  empty,
  emptyMessage = 'Nothing here yet.',
  loadingFallback,
  children,
}) {
  if (query.isLoading) {
    return (
      <Layout>
        <div className="p-8">
          <Header title={title} />
          {loadingFallback ?? <DefaultLoadingFallback />}
        </div>
      </Layout>
    );
  }

  if (query.isError) {
    return (
      <Layout>
        <div className="p-8 max-w-2xl">
          <Header title={title} />
          <div
            role="alert"
            className="rounded-md border border-accent-danger/30 bg-accent-danger/5 p-4 space-y-3"
          >
            <p className="text-sm text-accent-danger">
              {query.error?.message || 'Something went wrong.'}
            </p>
            <button
              type="button"
              onClick={() => query.refetch()}
              className="inline-flex items-center rounded-md bg-brand-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-brand-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2"
            >
              Try again
            </button>
          </div>
        </div>
      </Layout>
    );
  }

  const data = query.data;

  if (empty && empty(data)) {
    return (
      <Layout>
        <div className="p-8">
          <Header title={title} />
          <div className="rounded-lg border border-border-default bg-surface-0 p-10 text-center">
            <p className="text-sm text-text-secondary">{emptyMessage}</p>
          </div>
        </div>
      </Layout>
    );
  }

  return children(data);
}
