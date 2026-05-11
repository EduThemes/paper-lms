import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { useParams, Navigate } from 'react-router-dom';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import useUnsavedChanges from '../hooks/useUnsavedChanges';
import { useCourseUI } from '../contexts/CourseUIContext';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import { useLiveRegion } from '../components/LiveRegion';
import ButtonEditor from '../components/settings/ButtonEditor';
import OverrideEditor from '../components/settings/OverrideEditor';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Separator } from '@/components/ui/separator';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

// All available CourseNav tabs in default order
const DEFAULT_NAV_TABS = [
  { id: 'home', label: 'Home' },
  { id: 'announcements', label: 'Announcements' },
  { id: 'assignments', label: 'Assignments' },
  { id: 'modules', label: 'Modules' },
  { id: 'grades', label: 'Grades' },
  { id: 'people', label: 'People' },
  { id: 'quizzes', label: 'Quizzes' },
  { id: 'discussions', label: 'Discussions' },
  { id: 'files', label: 'Files' },
  { id: 'pages', label: 'Pages' },
  { id: 'rubrics', label: 'Rubrics' },
  { id: 'outcomes', label: 'Outcomes' },
  { id: 'groups', label: 'Groups' },
  { id: 'collaborations', label: 'Collaborations' },
  { id: 'conferences', label: 'Conferences' },
  { id: 'syllabus', label: 'Syllabus' },
  { id: 'attendance', label: 'Attendance' },
  { id: 'calendar', label: 'Calendar' },
  { id: 'question_banks', label: 'Question Banks' },
  { id: 'accommodations', label: 'Accommodations' },
  { id: 'blueprint', label: 'Blueprint' },
  { id: 'pacing', label: 'Pacing' },
  { id: 'analytics', label: 'Analytics' },
  { id: 'audit_log', label: 'Audit Log' },
  { id: 'content_import', label: 'Import Content' },
  { id: 'external_tools', label: 'External Tools' },
  { id: 'settings', label: 'Settings' },
];

// Tabs in the order Canvas's _settings.html.erb lays them out, plus our
// existing Navigation + Grading panels. The bottom three are link-only
// tabs: their content is owned by other pages, the tab here is a curated
// landing pad with a deep link so admins can find them without hunting
// through the global admin sidebar.
const TABS = [
  { id: 'general',     label: 'General' },
  { id: 'navigation',  label: 'Navigation' },
  { id: 'grading',     label: 'Grading' },
  { id: 'apps',        label: 'Apps' },
  { id: 'features',    label: 'Feature Options' },
  { id: 'integrations', label: 'Integrations' },
];

const DEFAULT_LATE_POLICY = {
  late_submission_deduction_enabled: false,
  late_submission_deduction: 0,
  late_submission_interval: 'day',
  late_submission_minimum_percent_enabled: false,
  late_submission_minimum_percent: 0,
  missing_submission_deduction_enabled: false,
  missing_submission_deduction: 0,
};

const DEFAULT_GRADING_SCALE = [
  ['A', 0.93], ['A-', 0.90], ['B+', 0.87], ['B', 0.83], ['B-', 0.80],
  ['C+', 0.77], ['C', 0.73], ['C-', 0.70], ['D+', 0.67], ['D', 0.63],
  ['D-', 0.60], ['F', 0.0],
];

const fmtDate = (d) => {
  if (!d) return '';
  const dt = new Date(d);
  return `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')}`;
};

const eq = (a, b) => JSON.stringify(a) === JSON.stringify(b);

// --- Local components -------------------------------------------------------

const SettingsTab = ({ tab, isActive, hasUnsaved, onSelect }) => (
  <button
    type="button"
    role="tab"
    id={`settings-tab-${tab.id}`}
    aria-controls={`settings-panel-${tab.id}`}
    aria-selected={isActive}
    aria-current={isActive ? 'page' : undefined}
    data-active={isActive}
    onClick={() => onSelect(tab.id)}
    className={cn(
      'group inline-flex items-center whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium',
      'text-text-secondary hover:text-text-primary hover:bg-surface-1',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
      'transition-colors duration-fast ease-emphatic',
      'data-[active=true]:bg-brand-50 data-[active=true]:text-brand-700'
    )}
  >
    {tab.label}
    {hasUnsaved && (
      <span
        aria-hidden="true"
        className="w-1.5 h-1.5 rounded-full bg-brand-500 ml-1.5"
      />
    )}
    {hasUnsaved && <span className="sr-only"> (unsaved changes)</span>}
  </button>
);

const SaveBar = ({ unsavedCount, saving, onSave, onDiscard, message }) => (
  <div
    role="region"
    aria-label="Unsaved changes"
    className={cn(
      'fixed bottom-0 inset-x-0 z-20 bg-surface-0 border-t border-border-default',
      'px-6 py-3 flex items-center justify-between shadow-md',
      'animate-in slide-in-from-bottom-4 duration-base ease-emphatic'
    )}
  >
    <div className="flex items-center gap-3 min-w-0">
      <span className="text-sm text-text-secondary">
        {unsavedCount} unsaved {unsavedCount === 1 ? 'change' : 'changes'}
      </span>
      {message && (
        <span
          className={cn(
            'text-sm truncate',
            message.startsWith('Error')
              ? 'text-accent-danger'
              : message.includes('failed')
                ? 'text-accent-warning'
                : 'text-accent-success'
          )}
        >
          {message}
        </span>
      )}
    </div>
    <div className="flex gap-2">
      <Button
        variant="outline"
        onClick={onDiscard}
        disabled={saving}
        aria-label="Discard unsaved changes"
      >
        Discard
      </Button>
      <Button
        onClick={onSave}
        aria-busy={saving}
        disabled={saving}
        aria-label={saving ? 'Saving changes' : 'Save changes'}
      >
        {saving ? 'Saving…' : 'Save changes'}
      </Button>
    </div>
  </div>
);

const SettingsLoadingSkeleton = () => (
  <Layout>
    <div className="mb-4">
      <Skeleton className="h-8 w-48" />
    </div>
    <div className="flex gap-1 mb-6 border-b border-border-default pb-2">
      <Skeleton className="h-9 w-20" />
      <Skeleton className="h-9 w-24" />
      <Skeleton className="h-9 w-20" />
    </div>
    <div className="space-y-6">
      {[0, 1, 2].map((i) => (
        <Card key={i}>
          <CardHeader>
            <Skeleton className="h-5 w-40" />
          </CardHeader>
          <CardContent className="space-y-3">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-3/4" />
          </CardContent>
        </Card>
      ))}
    </div>
  </Layout>
);

// --- Main page --------------------------------------------------------------

const CourseSettingsPage = () => {
  const { courseId } = useParams();
  const { course, setCourse } = useCourseUI();
  const isTeacher = useIsTeacher(courseId);
  const { announce } = useLiveRegion();

  const [form, setForm] = useState({
    name: '', course_code: '', default_view: 'modules', ui_mode: 'standard',
    license: 'private', is_public: false, start_at: '', end_at: '',
    apply_assignment_group_weights: false,
  });
  const [navTabs, setNavTabs] = useState(null);
  const [buttons, setButtons] = useState([]);
  const [overrides, setOverrides] = useState([]);
  const [latePolicy, setLatePolicy] = useState(DEFAULT_LATE_POLICY);
  const [latePolicyExists, setLatePolicyExists] = useState(false);
  const [gradingScale, setGradingScale] = useState(DEFAULT_GRADING_SCALE);
  const [gradingStandardId, setGradingStandardId] = useState(null);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [activeTab, setActiveTab] = useState('general');

  // Baseline snapshots for change detection
  const baselineRef = useRef({
    form: null,
    navTabs: null,
    latePolicy: null,
    gradingScale: null,
  });

  const captureBaseline = useCallback(() => {
    baselineRef.current = {
      form: JSON.parse(JSON.stringify(form)),
      navTabs: JSON.parse(JSON.stringify(navTabs)),
      latePolicy: JSON.parse(JSON.stringify(latePolicy)),
      gradingScale: JSON.parse(JSON.stringify(gradingScale)),
    };
  }, [form, navTabs, latePolicy, gradingScale]);

  // Initialize from course
  useEffect(() => {
    if (!course) return;
    const nextForm = {
      name: course.name || '',
      course_code: course.course_code || '',
      default_view: course.default_view || 'modules',
      ui_mode: course.ui_mode || 'standard',
      license: course.license || 'private',
      is_public: course.is_public || false,
      start_at: fmtDate(course.start_at),
      end_at: fmtDate(course.end_at),
      apply_assignment_group_weights: course.apply_assignment_group_weights || false,
    };
    setForm(nextForm);

    let nextNavTabs;
    if (Array.isArray(course.navigation_tabs) && course.navigation_tabs.length > 0) {
      const savedIds = new Set(course.navigation_tabs.map((t) => t.id));
      nextNavTabs = [
        ...course.navigation_tabs,
        ...DEFAULT_NAV_TABS
          .filter((t) => !savedIds.has(t.id))
          .map((t, i) => ({ id: t.id, hidden: false, position: course.navigation_tabs.length + i })),
      ];
    } else {
      nextNavTabs = DEFAULT_NAV_TABS.map((t, i) => ({ id: t.id, hidden: false, position: i }));
    }
    setNavTabs(nextNavTabs);

    baselineRef.current.form = JSON.parse(JSON.stringify(nextForm));
    baselineRef.current.navTabs = JSON.parse(JSON.stringify(nextNavTabs));
  }, [course]);

  useEffect(() => {
    if (!courseId) return;
    api.getCourseHomeButtons(courseId).then((r) => setButtons(r.data || [])).catch(() => {});
    api.getTodaysLessonOverrides(courseId).then((r) => setOverrides(r.data || [])).catch(() => {});
    api.getLatePolicy(courseId).then((data) => {
      let lp = null;
      if (data && data.late_policy) lp = data.late_policy;
      else if (data && data.id) lp = data;
      if (lp) {
        setLatePolicy(lp);
        setLatePolicyExists(true);
        baselineRef.current.latePolicy = JSON.parse(JSON.stringify(lp));
      } else {
        baselineRef.current.latePolicy = JSON.parse(JSON.stringify(DEFAULT_LATE_POLICY));
      }
    }).catch(() => {
      baselineRef.current.latePolicy = JSON.parse(JSON.stringify(DEFAULT_LATE_POLICY));
    });
    api.getGradingStandards(courseId).then((standards) => {
      if (Array.isArray(standards) && standards.length > 0) {
        const latest = standards[standards.length - 1];
        setGradingStandardId(latest.id);
        if (Array.isArray(latest.data)) {
          setGradingScale(latest.data);
          baselineRef.current.gradingScale = JSON.parse(JSON.stringify(latest.data));
          return;
        }
      }
      baselineRef.current.gradingScale = JSON.parse(JSON.stringify(DEFAULT_GRADING_SCALE));
    }).catch(() => {
      baselineRef.current.gradingScale = JSON.parse(JSON.stringify(DEFAULT_GRADING_SCALE));
    });
  }, [courseId]);

  // Per-tab dirty detection
  const dirtyByTab = useMemo(() => {
    const base = baselineRef.current;
    const generalKeys = ['name', 'course_code', 'default_view', 'ui_mode', 'license',
      'is_public', 'start_at', 'end_at'];
    const generalChanged = base.form
      ? generalKeys.some((k) => form[k] !== base.form[k])
      : false;
    const navigationChanged = base.navTabs && navTabs ? !eq(navTabs, base.navTabs) : false;
    const gradingChanged =
      (base.gradingScale ? !eq(gradingScale, base.gradingScale) : false) ||
      (base.latePolicy ? !eq(latePolicy, base.latePolicy) : false) ||
      (base.form ? form.apply_assignment_group_weights !== base.form.apply_assignment_group_weights : false);
    return {
      general: generalChanged,
      navigation: navigationChanged,
      grading: gradingChanged,
    };
  }, [form, navTabs, latePolicy, gradingScale]);

  const unsavedCount =
    (dirtyByTab.general ? 1 : 0) +
    (dirtyByTab.navigation ? 1 : 0) +
    (dirtyByTab.grading ? 1 : 0);
  const isDirty = unsavedCount > 0;
  useUnsavedChanges(isDirty);

  const updateForm = (patch) => setForm((f) => ({ ...f, ...patch }));
  const updateLatePolicy = (patch) => setLatePolicy((p) => ({ ...p, ...patch }));

  const handleDiscard = () => {
    const base = baselineRef.current;
    if (base.form) setForm(JSON.parse(JSON.stringify(base.form)));
    if (base.navTabs) setNavTabs(JSON.parse(JSON.stringify(base.navTabs)));
    if (base.latePolicy) setLatePolicy(JSON.parse(JSON.stringify(base.latePolicy)));
    if (base.gradingScale) setGradingScale(JSON.parse(JSON.stringify(base.gradingScale)));
    setMessage('');
    announce('Changes discarded.');
  };

  const handleSave = async () => {
    setSaving(true);
    setMessage('');
    try {
      const payload = { ...form };
      payload.start_at = payload.start_at ? new Date(payload.start_at).toISOString() : null;
      payload.end_at = payload.end_at ? new Date(payload.end_at).toISOString() : null;
      if (navTabs) payload.navigation_tabs = navTabs.map((t, i) => ({ ...t, position: i }));
      const updated = await api.updateCourse(courseId, payload);
      setCourse(updated);

      const saveErrors = [];
      try {
        if (latePolicyExists) {
          await api.updateLatePolicy(courseId, latePolicy);
        } else {
          await api.createLatePolicy(courseId, latePolicy);
          setLatePolicyExists(true);
        }
      } catch {
        try {
          if (!latePolicyExists) {
            await api.createLatePolicy(courseId, latePolicy);
            setLatePolicyExists(true);
          } else {
            saveErrors.push('late policy');
          }
        } catch {
          saveErrors.push('late policy');
        }
      }
      try {
        if (gradingStandardId) {
          await api.updateGradingStandard(courseId, gradingStandardId, 'Course Grading Scale', gradingScale);
        } else {
          const created = await api.createGradingStandard(courseId, 'Course Grading Scale', gradingScale);
          if (created?.id) setGradingStandardId(created.id);
        }
      } catch {
        saveErrors.push('grading scale');
      }

      captureBaseline();

      if (saveErrors.length > 0) {
        const msg = `Settings saved, but failed to save: ${saveErrors.join(', ')}. Please try again.`;
        setMessage(msg);
        announce(msg, 'assertive');
      } else {
        setMessage('Settings saved.');
        announce('Course settings saved.');
      }
    } catch (err) {
      setMessage('Error: ' + err.message);
      announce(`Error saving course settings: ${err.message}`, 'assertive');
    } finally {
      setSaving(false);
    }
  };

  const viewOptions = [
    { value: 'modules', label: 'Modules' },
    { value: 'syllabus', label: 'Syllabus' },
    { value: 'wiki', label: 'Front Page' },
    { value: 'announcements', label: 'Announcements' },
    { value: 'home_engine', label: 'Home Engine' },
  ];
  const modeOptions = [
    { value: 'standard', label: 'Standard', desc: 'Full LMS experience' },
    { value: 'k2', label: 'K-2', desc: 'Icons only, large touch targets, no text labels' },
    { value: '3-5', label: '3-5', desc: 'Icons + text, larger UI, simplified navigation' },
  ];

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null || navTabs === null) return <SettingsLoadingSkeleton />;

  return (
    <TooltipProvider delayDuration={200}>
      <Layout>
        <div className="mb-4">
          <h2 className="text-2xl font-bold">Course Settings</h2>
        </div>
        <CourseNav />

        {/* Tab strip: horizontal scroll on mobile, vertical sidebar on md+. */}
        <div className="md:hidden sticky top-0 z-10 bg-surface-0/95 backdrop-blur border-b border-border-default mb-6 -mx-2 px-2">
          <div
            role="tablist"
            aria-label="Settings sections"
            className="flex gap-1 overflow-x-auto py-2"
          >
            {TABS.map((tab) => (
              <SettingsTab
                key={tab.id}
                tab={tab}
                isActive={activeTab === tab.id}
                hasUnsaved={dirtyByTab[tab.id]}
                onSelect={setActiveTab}
              />
            ))}
          </div>
        </div>

        <div className="md:flex md:gap-6">
          <aside
            className="hidden md:block w-48 flex-shrink-0 sticky top-4 self-start"
            role="tablist"
            aria-label="Settings sections"
          >
            <nav className="flex flex-col gap-1">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  role="tab"
                  id={`settings-tab-${tab.id}-side`}
                  aria-controls={`settings-panel-${tab.id}`}
                  aria-selected={activeTab === tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={cn(
                    'flex items-center justify-between rounded-md px-3 py-2 text-sm font-medium text-left transition-colors',
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
                    activeTab === tab.id
                      ? 'bg-brand-50 text-brand-700'
                      : 'text-text-secondary hover:bg-surface-1 hover:text-text-primary'
                  )}
                >
                  <span>{tab.label}</span>
                  {dirtyByTab[tab.id] && (
                    <span aria-hidden="true" className="w-1.5 h-1.5 rounded-full bg-brand-500" />
                  )}
                </button>
              ))}
            </nav>
          </aside>

          <div className={cn('flex-1 min-w-0', isDirty && 'pb-20')}>
          {/* General */}
          {activeTab === 'general' && (
            <div
              role="tabpanel"
              id="settings-panel-general"
              aria-labelledby="settings-tab-general"
              className="space-y-6"
            >
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Course Details</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div className="space-y-1.5">
                      <Label htmlFor="course-name">Course Name</Label>
                      <Input
                        id="course-name"
                        value={form.name}
                        onChange={(e) => updateForm({ name: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="course-code">Course Code</Label>
                      <Input
                        id="course-code"
                        value={form.course_code}
                        onChange={(e) => updateForm({ course_code: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="course-start">Start Date</Label>
                      <Input
                        id="course-start"
                        type="date"
                        value={form.start_at}
                        onChange={(e) => updateForm({ start_at: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="course-end">End Date</Label>
                      <Input
                        id="course-end"
                        type="date"
                        value={form.end_at}
                        onChange={(e) => updateForm({ end_at: e.target.value })}
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="course-license">License</Label>
                      <select
                        id="course-license"
                        className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        value={form.license}
                        onChange={(e) => updateForm({ license: e.target.value })}
                      >
                        <option value="private">Private</option>
                        <option value="cc_by">CC BY</option>
                        <option value="cc_by_sa">CC BY-SA</option>
                        <option value="public_domain">Public Domain</option>
                      </select>
                    </div>
                    <div className="flex items-center gap-2 pt-7">
                      <input
                        id="course-public"
                        type="checkbox"
                        checked={form.is_public}
                        onChange={(e) => updateForm({ is_public: e.target.checked })}
                      />
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Label htmlFor="course-public" className="cursor-pointer">
                            Public Course
                          </Label>
                        </TooltipTrigger>
                        <TooltipContent>
                          Anyone with the link can view this course.
                        </TooltipContent>
                      </Tooltip>
                    </div>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Home Page</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2">
                    {viewOptions.map((opt) => (
                      <label key={opt.value} className="flex items-center gap-2">
                        <input
                          type="radio"
                          name="default_view"
                          value={opt.value}
                          checked={form.default_view === opt.value}
                          onChange={(e) => updateForm({ default_view: e.target.value })}
                        />
                        <span className="text-sm">{opt.label}</span>
                      </label>
                    ))}
                  </div>
                  {form.default_view === 'home_engine' && (
                    <div className="mt-6 space-y-6">
                      <Separator />
                      <div>
                        <h4 className="font-medium text-sm text-text-secondary mb-3">Home Buttons</h4>
                        <ButtonEditor courseId={courseId} buttons={buttons} setButtons={setButtons} />
                      </div>
                      <div>
                        <h4 className="font-medium text-sm text-text-secondary mb-3">Today&apos;s Lesson Overrides</h4>
                        <OverrideEditor courseId={courseId} overrides={overrides} setOverrides={setOverrides} />
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">UI Mode</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {modeOptions.map((opt) => (
                      <label
                        key={opt.value}
                        className={cn(
                          'border-2 rounded-lg p-4 cursor-pointer transition-colors duration-fast ease-emphatic',
                          form.ui_mode === opt.value
                            ? 'border-brand-500 bg-brand-50'
                            : 'border-border-default hover:border-border-strong'
                        )}
                      >
                        <input
                          type="radio"
                          name="ui_mode"
                          value={opt.value}
                          className="sr-only"
                          checked={form.ui_mode === opt.value}
                          onChange={(e) => updateForm({ ui_mode: e.target.value })}
                        />
                        <div className="font-semibold">{opt.label}</div>
                        <div className="text-xs text-text-tertiary mt-1">{opt.desc}</div>
                      </label>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Navigation */}
          {activeTab === 'navigation' && navTabs && (
            <div
              role="tabpanel"
              id="settings-panel-navigation"
              aria-labelledby="settings-tab-navigation"
              className="space-y-6"
            >
              <Card>
                <CardContent className="pt-6">
                  <p className="text-sm text-text-secondary mb-4">
                    Reorder tabs using the arrows. Toggle visibility to show or hide tabs from students. The first 6 visible tabs appear in the primary navigation bar; the rest appear under &quot;More&quot;.
                  </p>
                  <div className="space-y-1">
                    {navTabs.map((tab, idx) => {
                      const def = DEFAULT_NAV_TABS.find((d) => d.id === tab.id);
                      const label = def ? def.label : tab.id;
                      return (
                        <div
                          key={tab.id}
                          className={cn(
                            'flex items-center gap-3 px-3 py-2 rounded-md border',
                            tab.hidden
                              ? 'bg-surface-1 border-border-subtle opacity-60'
                              : 'bg-surface-0 border-border-default'
                          )}
                        >
                          <span className="text-xs text-text-tertiary w-5 text-right">{idx + 1}</span>
                          <div className="flex flex-col gap-0.5">
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <button
                                  type="button"
                                  disabled={idx === 0}
                                  className="text-text-secondary hover:text-text-primary disabled:opacity-30 disabled:cursor-not-allowed p-0.5"
                                  aria-label={`Move ${label} up`}
                                  onClick={() => {
                                    const updated = [...navTabs];
                                    [updated[idx - 1], updated[idx]] = [updated[idx], updated[idx - 1]];
                                    setNavTabs(updated);
                                  }}
                                >
                                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 15l7-7 7 7" />
                                  </svg>
                                </button>
                              </TooltipTrigger>
                              <TooltipContent>Move up</TooltipContent>
                            </Tooltip>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <button
                                  type="button"
                                  disabled={idx === navTabs.length - 1}
                                  className="text-text-secondary hover:text-text-primary disabled:opacity-30 disabled:cursor-not-allowed p-0.5"
                                  aria-label={`Move ${label} down`}
                                  onClick={() => {
                                    const updated = [...navTabs];
                                    [updated[idx], updated[idx + 1]] = [updated[idx + 1], updated[idx]];
                                    setNavTabs(updated);
                                  }}
                                >
                                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                                  </svg>
                                </button>
                              </TooltipTrigger>
                              <TooltipContent>Move down</TooltipContent>
                            </Tooltip>
                          </div>
                          <span className={cn(
                            'flex-1 text-sm font-medium',
                            tab.hidden ? 'text-text-tertiary line-through' : 'text-text-primary'
                          )}>
                            {label}
                          </span>
                          <label className="relative inline-flex items-center cursor-pointer">
                            <input
                              type="checkbox"
                              className="sr-only peer"
                              checked={!tab.hidden}
                              aria-label={`${tab.hidden ? 'Show' : 'Hide'} ${label}`}
                              onChange={() => {
                                setNavTabs(navTabs.map((t, i) =>
                                  i === idx ? { ...t, hidden: !t.hidden } : t
                                ));
                              }}
                            />
                            <div className="w-9 h-5 bg-surface-2 peer-focus:outline-none peer-focus:ring-2 peer-focus:ring-ring rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-surface-0 after:border-border-default after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-brand-600"></div>
                            <span className="ml-2 text-xs text-text-tertiary">{tab.hidden ? 'Hidden' : 'Visible'}</span>
                          </label>
                        </div>
                      );
                    })}
                  </div>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Grading */}
          {activeTab === 'grading' && (
            <div
              role="tabpanel"
              id="settings-panel-grading"
              aria-labelledby="settings-tab-grading"
              className="space-y-6"
            >
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Grading Scale</CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="text-sm text-text-secondary mb-4">
                    Customize the letter grade boundaries for this course. Percentages are minimum thresholds (e.g., 93% means scores of 93% and above earn that grade).
                  </p>
                  <div className="space-y-2">
                    {gradingScale.map(([name, value], idx) => (
                      <div key={idx} className="flex items-center gap-3">
                        <Input
                          className="w-16 text-center font-medium"
                          value={name}
                          aria-label={`Grade ${idx + 1} letter`}
                          onChange={(e) => {
                            const updated = [...gradingScale];
                            updated[idx] = [e.target.value, value];
                            setGradingScale(updated);
                          }}
                        />
                        <span className="text-sm text-text-tertiary">&ge;</span>
                        <Input
                          type="number"
                          min="0"
                          max="100"
                          step="1"
                          className="w-20 text-center"
                          value={Math.round(value * 100)}
                          aria-label={`Grade ${idx + 1} minimum percent`}
                          onChange={(e) => {
                            const updated = [...gradingScale];
                            updated[idx] = [name, (parseFloat(e.target.value) || 0) / 100];
                            setGradingScale(updated);
                          }}
                        />
                        <span className="text-sm text-text-tertiary">%</span>
                        {gradingScale.length > 2 && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            className="text-accent-danger hover:text-accent-danger hover:bg-accent-danger/10"
                            onClick={() => setGradingScale(gradingScale.filter((_, i) => i !== idx))}
                          >
                            Remove
                          </Button>
                        )}
                      </div>
                    ))}
                    <Button
                      type="button"
                      variant="link"
                      size="sm"
                      className="mt-2 px-0"
                      onClick={() => {
                        const last = gradingScale[gradingScale.length - 1];
                        const newValue = last ? Math.max(last[1] - 0.05, 0) : 0;
                        setGradingScale([
                          ...gradingScale.slice(0, -1),
                          ['New', newValue],
                          gradingScale[gradingScale.length - 1],
                        ]);
                      }}
                    >
                      + Add grade level
                    </Button>
                  </div>
                  <Separator className="my-4" />
                  <div className="flex items-center gap-2">
                    <input
                      id="weight-groups"
                      type="checkbox"
                      checked={form.apply_assignment_group_weights}
                      onChange={(e) => updateForm({ apply_assignment_group_weights: e.target.checked })}
                    />
                    <Label htmlFor="weight-groups" className="cursor-pointer">
                      Weight final grade based on assignment groups
                    </Label>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardContent className="pt-6">
                  <fieldset>
                    <legend className="font-semibold text-lg mb-4">Grading Rules: Late &amp; Missing Work</legend>
                    <div className="divide-y divide-border-subtle">
                      <div className="py-4 first:pt-0">
                        <div className="flex items-center gap-2 mb-3">
                          <input
                            type="checkbox"
                            id="missing-deduction-enabled"
                            checked={latePolicy.missing_submission_deduction_enabled}
                            onChange={(e) => updateLatePolicy({ missing_submission_deduction_enabled: e.target.checked })}
                          />
                          <Label htmlFor="missing-deduction-enabled" className="cursor-pointer">
                            Automatically apply a grade for missing submissions
                          </Label>
                        </div>
                        {latePolicy.missing_submission_deduction_enabled && (
                          <div className="ml-6 flex items-center gap-2 text-sm text-text-secondary">
                            <span>Grade missing submissions as</span>
                            <Input
                              type="number"
                              min="0"
                              max="100"
                              step="1"
                              className="w-20"
                              value={100 - (latePolicy.missing_submission_deduction || 0)}
                              aria-label="Missing submission grade percent"
                              onChange={(e) => updateLatePolicy({
                                missing_submission_deduction: 100 - (parseFloat(e.target.value) || 0),
                              })}
                            />
                            <span>% of possible points</span>
                          </div>
                        )}
                      </div>
                      <div className="py-4">
                        <div className="flex items-center gap-2 mb-3">
                          <input
                            type="checkbox"
                            id="late-deduction-enabled"
                            checked={latePolicy.late_submission_deduction_enabled}
                            onChange={(e) => updateLatePolicy({ late_submission_deduction_enabled: e.target.checked })}
                          />
                          <Label htmlFor="late-deduction-enabled" className="cursor-pointer">
                            Automatically deduct from late submissions
                          </Label>
                        </div>
                        {latePolicy.late_submission_deduction_enabled && (
                          <div className="ml-6 flex flex-wrap items-center gap-2 text-sm text-text-secondary">
                            <span>Deduct</span>
                            <Input
                              type="number"
                              min="0"
                              max="100"
                              step="1"
                              className="w-20"
                              value={latePolicy.late_submission_deduction}
                              aria-label="Late deduction percent"
                              onChange={(e) => updateLatePolicy({
                                late_submission_deduction: parseFloat(e.target.value) || 0,
                              })}
                            />
                            <span>% for each late</span>
                            <select
                              className="flex h-10 rounded-md border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                              value={latePolicy.late_submission_interval}
                              aria-label="Late deduction interval"
                              onChange={(e) => updateLatePolicy({ late_submission_interval: e.target.value })}
                            >
                              <option value="day">day</option>
                              <option value="hour">hour</option>
                            </select>
                          </div>
                        )}
                      </div>
                      <div className="py-4 last:pb-0">
                        <div className="flex items-center gap-2 mb-3">
                          <input
                            type="checkbox"
                            id="min-percent-enabled"
                            checked={latePolicy.late_submission_minimum_percent_enabled}
                            onChange={(e) => updateLatePolicy({ late_submission_minimum_percent_enabled: e.target.checked })}
                          />
                          <Label htmlFor="min-percent-enabled" className="cursor-pointer">
                            Set a minimum grade for late submissions
                          </Label>
                        </div>
                        {latePolicy.late_submission_minimum_percent_enabled && (
                          <div className="ml-6 flex items-center gap-2 text-sm text-text-secondary">
                            <span>Minimum grade allowed:</span>
                            <Input
                              type="number"
                              min="0"
                              max="100"
                              step="1"
                              className="w-20"
                              value={latePolicy.late_submission_minimum_percent}
                              aria-label="Minimum grade percent"
                              onChange={(e) => updateLatePolicy({
                                late_submission_minimum_percent: parseFloat(e.target.value) || 0,
                              })}
                            />
                            <span>%</span>
                          </div>
                        )}
                      </div>
                    </div>
                  </fieldset>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Apps — link panel: external tools live on their own page. */}
          {activeTab === 'apps' && (
            <div
              role="tabpanel"
              id="settings-panel-apps"
              aria-labelledby="settings-tab-apps"
              className="space-y-4"
            >
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Apps & External Tools</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <p className="text-sm text-text-secondary">
                    LTI tools, content publishers, and third-party integrations are managed on the
                    External Tools page. Add a tool there and it becomes available in the Rich Content
                    Editor and module item picker.
                  </p>
                  <Button asChild variant="outline">
                    <a href={`/courses/${courseId}/external_tools`}>Open External Tools</a>
                  </Button>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Feature Options — link panel: course-scoped feature flags. */}
          {activeTab === 'features' && (
            <div
              role="tabpanel"
              id="settings-panel-features"
              aria-labelledby="settings-tab-features"
              className="space-y-4"
            >
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Feature Options</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <p className="text-sm text-text-secondary">
                    Toggle in-progress LMS features for this course only. Account-level admins can
                    set the default for every course; here you can opt this course in or out.
                  </p>
                  <Button asChild variant="outline">
                    <a href={`/admin/feature_flags?context=course&id=${courseId}`}>
                      Open Feature Flags
                    </a>
                  </Button>
                </CardContent>
              </Card>
            </div>
          )}

          {/* Integrations — link panel: developer keys + roster sync. */}
          {activeTab === 'integrations' && (
            <div
              role="tabpanel"
              id="settings-panel-integrations"
              aria-labelledby="settings-tab-integrations"
              className="space-y-4"
            >
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">Integrations</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <p className="text-sm text-text-secondary">
                    OAuth and LTI 1.3 developer keys, OneRoster CSV/REST sync, and SIS imports are
                    account-wide and live under Admin. Course-level overrides aren't supported.
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Button asChild variant="outline">
                      <a href="/admin/developer_keys">Developer Keys</a>
                    </Button>
                    <Button asChild variant="outline">
                      <a href="/admin/oneroster">OneRoster</a>
                    </Button>
                    <Button asChild variant="outline">
                      <a href="/admin/sis_import">SIS Import</a>
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </div>
          )}
          </div>
        </div>

        {isDirty && (
          <SaveBar
            unsavedCount={unsavedCount}
            saving={saving}
            onSave={handleSave}
            onDiscard={handleDiscard}
            message={message}
          />
        )}
      </Layout>
    </TooltipProvider>
  );
};

export default CourseSettingsPage;
