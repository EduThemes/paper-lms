import React from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { BarChart3, FileQuestion, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useIsTeacher from '../hooks/useIsTeacher';
import Page from '../components/Page';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import { TYPE_LABELS } from '../components/quiz/itemTypes/types';
import { quizItemsApi, quizItemsKeys } from '../services/api_quiz_items';

/**
 * Item Analysis — per-question statistics over each student's latest
 * finished attempt, computed server-side (GET .../item_analysis):
 * difficulty, discrimination (top vs bottom 27%), point-biserial, answer
 * distributions for every item type, Cronbach's alpha, plus item- and
 * student-analysis CSV downloads.
 */

const fmt = (v, digits = 2) => (v == null || Number.isNaN(Number(v)) ? '—' : Number(v).toFixed(digits));
const pct = (v) => (v == null ? '—' : `${Math.round(Number(v) * 100)}%`);

// Plain-language reading of the indices (thresholds commonly used in
// classical test theory).
export function difficultyLabel(d) {
  if (d == null) return '';
  if (d >= 0.9) return 'very easy';
  if (d >= 0.7) return 'easy';
  if (d >= 0.3) return 'moderate';
  return 'hard';
}

export function discriminationLabel(d) {
  if (d == null) return '';
  if (d >= 0.4) return 'good';
  if (d >= 0.2) return 'fair';
  if (d >= 0) return 'poor';
  return 'negative — review this item';
}

// Groups options/responses by their blank / left-hand item.
export function groupOptions(options = []) {
  const groups = [];
  const byName = new Map();
  for (const o of options) {
    const key = o.group || '';
    if (!byName.has(key)) {
      const g = { name: key, options: [] };
      byName.set(key, g);
      groups.push(g);
    }
    byName.get(key).options.push(o);
  }
  return groups;
}

const Stat = ({ label, value, hint }) => (
  <div className="bg-surface-1 rounded-lg p-3">
    <div className="text-xs text-text-tertiary uppercase tracking-wide">{label}</div>
    <div className="text-lg font-semibold text-text-primary mt-0.5 tabular-nums">{value}</div>
    {hint && <div className="text-xs text-text-tertiary">{hint}</div>}
  </div>
);

const Distribution = ({ options, respondents }) => (
  <div className="mt-3 space-y-3">
    {groupOptions(options).map(group => (
      <div key={group.name || 'all'}>
        {group.name && <div className="text-xs font-medium text-text-secondary mb-1">{group.name}</div>}
        <div className="space-y-1.5">
          {group.options.map((o, i) => {
            const share = respondents > 0 ? Math.round((o.count / respondents) * 100) : 0;
            return (
              <div key={`${o.id || o.text}-${i}`} className="flex items-center gap-2 text-xs">
                <span className={`flex-1 truncate ${o.correct ? 'font-semibold text-accent-success' : 'text-text-secondary'}`}>
                  {o.correct && <span className="sr-only">Correct: </span>}{o.text || '(blank)'}
                </span>
                <div className="w-32 h-2 bg-surface-1 rounded overflow-hidden" aria-hidden="true">
                  <div className={`h-full ${o.correct ? 'bg-accent-success/70' : 'bg-text-tertiary/50'}`} style={{ width: `${share}%` }} />
                </div>
                <span className="w-16 text-right text-text-tertiary tabular-nums">{o.count} ({share}%)</span>
                {o.point_biserial != null && (
                  <span className="w-16 text-right text-text-tertiary tabular-nums" title="Point-biserial for choosing this option">r {fmt(o.point_biserial)}</span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    ))}
  </div>
);

const ItemCard = ({ item, index }) => {
  const options = [...(item.options || []), ...(item.responses || [])];
  const difficulty = item.difficulty_index;
  const barPct = Math.max(0, Math.min(100, Math.round((difficulty ?? 0) * 100)));
  return (
    <section className="bg-surface-0 rounded-lg shadow border border-border-default p-5" aria-label={`Question ${index + 1}`}>
      <header className="flex items-start justify-between mb-3 gap-3">
        <div className="min-w-0">
          <div className="text-xs text-text-tertiary uppercase tracking-wide">
            Q{index + 1} · {TYPE_LABELS[item.question_type] || item.question_type}
          </div>
          <div className="text-sm text-text-primary mt-1 line-clamp-2">{item.question_text}</div>
        </div>
        <div className="text-right text-xs text-text-tertiary whitespace-nowrap">
          <div>{item.respondents} respondents · {item.unanswered} unanswered</div>
          <div className="mt-0.5">avg {fmt(item.average_points)} / {item.points_possible} pts</div>
          {item.pending_review > 0 && <div className="text-accent-warning mt-0.5">{item.pending_review} pending review</div>}
        </div>
      </header>

      {difficulty != null && (
        <div className="flex items-center gap-3 mb-3">
          <div className="flex-1 h-3 bg-surface-1 rounded overflow-hidden" aria-hidden="true">
            <div className={`h-full ${barPct >= 70 ? 'bg-accent-success' : barPct >= 40 ? 'bg-accent-warning' : 'bg-accent-danger'}`} style={{ width: `${barPct}%` }} />
          </div>
          <span className="text-sm font-semibold text-text-primary w-12 text-right">{barPct}%</span>
        </div>
      )}

      <dl className="grid grid-cols-2 sm:grid-cols-5 gap-2 text-xs">
        <div><dt className="text-text-tertiary">Correct</dt><dd className="text-text-primary tabular-nums">{item.correct}{item.partially_correct > 0 ? ` (+${item.partially_correct} partial)` : ''}</dd></div>
        <div><dt className="text-text-tertiary">Incorrect</dt><dd className="text-text-primary tabular-nums">{item.incorrect}</dd></div>
        <div><dt className="text-text-tertiary">Difficulty</dt><dd className="text-text-primary tabular-nums">{fmt(difficulty)} <span className="text-text-tertiary">{difficultyLabel(difficulty)}</span></dd></div>
        <div><dt className="text-text-tertiary">Discrimination</dt><dd className="text-text-primary tabular-nums">{fmt(item.discrimination_index)} <span className="text-text-tertiary">{discriminationLabel(item.discrimination_index)}</span></dd></div>
        <div><dt className="text-text-tertiary">Point-biserial</dt><dd className="text-text-primary tabular-nums">{fmt(item.point_biserial)}</dd></div>
      </dl>

      {options.length > 0 && <Distribution options={options} respondents={item.respondents} />}
    </section>
  );
};

const ItemAnalysisPage = () => {
  const { t } = useTranslation();
  const { courseId, quizId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const query = useQuery({
    queryKey: quizItemsKeys.itemAnalysis(courseId, quizId),
    queryFn: () => quizItemsApi.getItemAnalysis(courseId, quizId),
    // The endpoint is instructor-only; a non-teacher is redirected below.
    enabled: isTeacher !== false,
  });

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  return (
    <Page query={query} title={t('itemAnalysis.title')}>
      {(report) => (
        <Layout>
          <CourseNav />
          <QuizzesSubNav quizId={quizId} />
          <header className="mb-6 flex flex-wrap items-end justify-between gap-3">
            <div>
              <Link to={`/courses/${courseId}/quizzes/${quizId}/edit`} className="text-brand-600 hover:underline text-sm">
                {t('itemAnalysis.backToQuiz')}
              </Link>
              <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2 mt-2">
                <BarChart3 className="w-6 h-6 text-brand-600" />
                {t('itemAnalysis.title')}
              </h1>
              <p className="text-sm text-text-tertiary mt-1">
                {report.quiz_title}
                {report.anonymous && <span className="ml-2 text-xs px-2 py-0.5 rounded-full bg-surface-2">Anonymous survey</span>}
              </p>
            </div>
            <div className="flex gap-2">
              <a href={quizItemsApi.itemAnalysisCsvUrl(courseId, quizId)} download
                className="inline-flex items-center gap-1 text-sm px-3 py-1.5 rounded border border-border-strong hover:bg-surface-1">
                <Download className="w-4 h-4" /> Item analysis CSV
              </a>
              <a href={quizItemsApi.studentAnalysisCsvUrl(courseId, quizId)} download
                className="inline-flex items-center gap-1 text-sm px-3 py-1.5 rounded border border-border-strong hover:bg-surface-1">
                <Download className="w-4 h-4" /> Student analysis CSV
              </a>
            </div>
          </header>

          {report.summary.respondents === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow border border-border-default p-8 text-center">
              <FileQuestion className="w-10 h-10 mx-auto text-text-disabled mb-3" />
              <h2 className="text-lg font-semibold text-text-primary mb-1">{t('itemAnalysis.noDataTitle')}</h2>
              <p className="text-sm text-text-tertiary max-w-md mx-auto">{t('itemAnalysis.noDataDescription')}</p>
              {report.items.length > 0 && (
                <p className="text-xs text-text-disabled mt-3">{t('itemAnalysis.questionsConfigured', { count: report.items.length })}</p>
              )}
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 md:grid-cols-6 gap-3 mb-6">
                <Stat label="Students" value={report.summary.respondents} hint="latest attempt each" />
                <Stat label="Mean score" value={`${fmt(report.summary.mean_score)} / ${report.summary.points_possible}`} />
                <Stat label="Median" value={fmt(report.summary.median_score)} />
                <Stat label="Std. deviation" value={fmt(report.summary.standard_deviation)} />
                <Stat label="High / low" value={`${report.summary.high_score} / ${report.summary.low_score}`} />
                <Stat label="Cronbach α" value={fmt(report.summary.alpha)} hint={report.summary.alpha_item_count ? `${report.summary.alpha_item_count} items` : 'needs 2+ shared items'} />
              </div>
              <div className="space-y-4">
                {report.items.map((item, i) => <ItemCard key={item.question_id} item={item} index={i} />)}
              </div>
              <p className="text-xs text-text-tertiary mt-4">
                Difficulty is the share of available points earned ({pct(0.7)}+ easy, under {pct(0.3)} hard).
                Discrimination compares the top and bottom 27% of students; above 0.4 is good, negative values mean stronger students missed it.
              </p>
            </>
          )}
        </Layout>
      )}
    </Page>
  );
};

export default ItemAnalysisPage;
