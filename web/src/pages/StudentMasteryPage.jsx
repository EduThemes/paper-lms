import React, { useMemo, useState } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ArrowLeft, Target, ChevronDown, ChevronRight, CheckCircle2 } from 'lucide-react';
import { masteryApi } from '../services/api_learner_outcomes';
import { describeCalculation } from '../utils/outcomeCalculation';
import { groupMasteryOutcomes, formatMasteryScore } from '../utils/studentMastery';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';

function RatingBadge({ rating }) {
  if (!rating) return <span className="text-xs text-text-tertiary">Not assessed</span>;
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium px-2 py-0.5 rounded-full border border-border-default">
      <span className="inline-block w-2.5 h-2.5 rounded-full" style={{ backgroundColor: rating.color }} aria-hidden="true" />
      {rating.description}
    </span>
  );
}

function OutcomeCard({ outcome }) {
  const [open, setOpen] = useState(false);
  const results = outcome.results || [];
  return (
    <li className="bg-surface-0 rounded-lg shadow">
      <div className="p-4 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h4 className="font-semibold text-text-primary flex items-center gap-2">
            {outcome.mastered && <CheckCircle2 className="w-4 h-4 text-accent-success" aria-label="Mastered" />}
            {outcome.display_name || outcome.title}
          </h4>
          {outcome.description && <p className="text-sm text-text-secondary mt-1 whitespace-pre-line">{outcome.description}</p>}
          <p className="text-xs text-text-tertiary mt-1">{describeCalculation(outcome.calculation_method, outcome.calculation_int)}</p>
        </div>
        <div className="text-right space-y-1">
          <RatingBadge rating={outcome.rating} />
          <p className="text-sm font-semibold text-text-primary">{formatMasteryScore(outcome.score, outcome.possible)}</p>
        </div>
      </div>
      {results.length > 0 && (
        <div className="border-t border-border-subtle">
          <button type="button" onClick={() => setOpen((o) => !o)} aria-expanded={open}
            className="w-full flex items-center gap-1 px-4 py-2 text-sm text-brand-600 hover:bg-surface-1">
            {open ? <ChevronDown className="w-4 h-4" aria-hidden="true" /> : <ChevronRight className="w-4 h-4" aria-hidden="true" />}
            {results.length} aligned result{results.length === 1 ? '' : 's'}
          </button>
          {open && (
            <ul className="divide-y divide-border-subtle">
              {results.map((r) => (
                <li key={r.id} className="px-4 py-2 flex flex-wrap items-center justify-between gap-2 text-sm">
                  <span className="text-text-primary">
                    {r.title || `${r.associated_asset_type || 'Item'} ${r.associated_asset_id}`}
                    <span className="text-xs text-text-tertiary ml-2">
                      {(r.submitted_at || r.assessed_at) ? new Date(r.submitted_at || r.assessed_at).toLocaleDateString() : ''}
                    </span>
                  </span>
                  <span className="flex items-center gap-3">
                    {r.raw_score != null && r.raw_possible != null && (
                      <span className="text-xs text-text-tertiary">{r.raw_score} / {r.raw_possible} pts</span>
                    )}
                    <RatingBadge rating={r.rating} />
                    <span className="font-medium">{formatMasteryScore(r.score, r.possible)}</span>
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </li>
  );
}

/**
 * Student Learning Mastery view (G152): the student's own mastery of every
 * course outcome — rating, score on the course proficiency scale, and the
 * aligned results behind it. Observers pass ?observee_id=, staff ?user_id=.
 */
export default function StudentMasteryPage() {
  const { courseId } = useParams();
  const [search] = useSearchParams();
  const target = search.get('user_id') || search.get('observee_id') || 'self';
  const query = useQuery({
    queryKey: ['mastery', String(courseId), String(target)],
    queryFn: () => masteryApi.studentMastery(courseId, target),
    retry: false,
  });
  const groups = useMemo(() => groupMasteryOutcomes(query.data?.outcomes || []), [query.data]);

  if (query.isError && query.error?.status === 404) {
    return (
      <Layout>
        <CourseNav />
        <div className="rounded-md border border-border-default bg-surface-0 p-6 text-text-secondary">
          Learning mastery is shown for students of this course.
        </div>
      </Layout>
    );
  }

  return (
    <Page query={query} title="Learning Mastery">
      {(data) => (
        <Layout>
          <CourseNav />
          <div className="mb-6">
            <Link to={`/courses/${courseId}/grades${target !== 'self' && search.get('observee_id') ? `?observee_id=${target}` : ''}`} className="text-brand-600 hover:underline text-sm flex items-center gap-1">
              <ArrowLeft className="w-3.5 h-3.5" aria-hidden="true" /> Grades
            </Link>
            <div className="flex items-center gap-3 mt-2">
              <Target className="w-7 h-7 text-brand-600" aria-hidden="true" />
              <div>
                <h2 className="text-2xl font-bold text-text-primary">Learning Mastery</h2>
                {target !== 'self' && data.user_name && <p className="text-sm text-text-tertiary">{data.user_name}</p>}
              </div>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-4 mb-6">
            <div className="bg-surface-0 rounded-lg shadow px-4 py-3">
              <p className="text-xs uppercase text-text-tertiary font-medium">Outcomes mastered</p>
              <p className="text-2xl font-bold text-text-primary">{data.mastered_count} <span className="text-base font-normal text-text-tertiary">of {(data.outcomes || []).length}</span></p>
            </div>
            <div className="flex flex-wrap items-center gap-3 text-sm" aria-label="Proficiency scale">
              {(data.proficiency?.ratings || []).map((r) => (
                <span key={r.id || r.description} className="flex items-center gap-1.5">
                  <span className="inline-block w-3 h-3 rounded-full" style={{ backgroundColor: r.color }} aria-hidden="true" />
                  <span className={r.mastery ? 'font-semibold' : ''}>{r.description}</span>
                </span>
              ))}
            </div>
          </div>

          {groups.length === 0 ? (
            <div className="rounded-md border border-dashed border-border-strong bg-surface-0 p-8 text-center text-text-tertiary">
              This course has no learning outcomes yet.
            </div>
          ) : (
            <div className="space-y-6">
              {groups.map((g) => (
                <section key={g.key} aria-labelledby={`mg-${g.key}`}>
                  <h3 id={`mg-${g.key}`} className="text-lg font-semibold text-text-primary mb-2">{g.title}</h3>
                  <ul className="space-y-3">
                    {g.outcomes.map((o) => <OutcomeCard key={o.id} outcome={o} />)}
                  </ul>
                </section>
              ))}
            </div>
          )}
        </Layout>
      )}
    </Page>
  );
}
