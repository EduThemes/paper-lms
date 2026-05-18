import React, { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate, Outlet } from 'react-router-dom';
const DashboardPage = React.lazy(() => import('./pages/DashboardPage'));
const CoursePage = React.lazy(() => import('./pages/CoursePage'));
const CoursesPage = React.lazy(() => import('./pages/CoursesPage'));
const AssignmentPage = React.lazy(() => import('./pages/AssignmentPage'));
const AssignmentsPage = React.lazy(() => import('./pages/AssignmentsPage'));
const GradebookPage = React.lazy(() => import('./pages/GradebookPage'));
const StudentGradesPage = React.lazy(() => import('./pages/StudentGradesPage'));
const ContentImportPage = React.lazy(() => import('./pages/ContentImportPage'));
const ModulesPage = React.lazy(() => import('./pages/ModulesPage'));
const PeoplePage = React.lazy(() => import('./pages/PeoplePage'));
const AccessTokensPage = React.lazy(() => import('./pages/AccessTokensPage'));
const DeveloperKeysPage = React.lazy(() => import('./pages/DeveloperKeysPage'));
const ExternalToolsPage = React.lazy(() => import('./pages/ExternalToolsPage'));
const DiscussionsPage = React.lazy(() => import('./pages/DiscussionsPage'));
const DiscussionTopicPage = React.lazy(() => import('./pages/DiscussionTopicPageV2'));
const FilesPage = React.lazy(() => import('./pages/FilesPage'));
const SISImportPage = React.lazy(() => import('./pages/SISImportPage'));
const PagesPage = React.lazy(() => import('./pages/PagesPage'));
const PageDetailPage = React.lazy(() => import('./pages/PageDetailPage'));
const QuizzesPage = React.lazy(() => import('./pages/QuizzesPage'));
const QuizTakePage = React.lazy(() => import('./pages/QuizTakePage'));
const QuizReviewPage = React.lazy(() => import('./pages/QuizReviewPage'));
const QuizEditorPage = React.lazy(() => import('./pages/QuizEditorPage'));
const QuizSubmissionsPage = React.lazy(() => import('./pages/QuizSubmissionsPage'));
const QuizStatisticsPage = React.lazy(() => import('./pages/QuizStatisticsPage'));
const RubricsPage = React.lazy(() => import('./pages/RubricsPage'));
const GradingPeriodsPage = React.lazy(() => import('./pages/GradingPeriodsPage'));
const AssignmentOverridesPage = React.lazy(() => import('./pages/AssignmentOverridesPage'));
const CalendarPage = React.lazy(() => import('./pages/CalendarPage'));
const InboxPage = React.lazy(() => import('./pages/InboxPage'));
const NotificationPreferencesPage = React.lazy(() => import('./pages/NotificationPreferencesPage'));
const NotificationsPage = React.lazy(() => import('./pages/NotificationsPage'));
const SpeedGraderPage = React.lazy(() => import('./pages/SpeedGraderPage'));
const LearningOutcomesPage = React.lazy(() => import('./pages/LearningOutcomesPage'));
const GroupsPage = React.lazy(() => import('./pages/GroupsPage'));
const BlueprintPage = React.lazy(() => import('./pages/BlueprintPage'));
const CoursePacingPage = React.lazy(() => import('./pages/CoursePacingPage'));
const CollaborationsPage = React.lazy(() => import('./pages/CollaborationsPage'));
const ConferencesPage = React.lazy(() => import('./pages/ConferencesPage'));
const AnalyticsPage = React.lazy(() => import('./pages/AnalyticsPage'));
const GraphiQLPage = React.lazy(() => import('./pages/GraphiQLPage'));
const AuthProvidersPage = React.lazy(() => import('./pages/AuthProvidersPage'));
const AnnouncementsPage = React.lazy(() => import('./pages/AnnouncementsPage'));
const EnrollmentTermsPage = React.lazy(() => import('./pages/EnrollmentTermsPage'));
const SyllabusPage = React.lazy(() => import('./pages/SyllabusPage'));
const NotificationDeliveryPage = React.lazy(() => import('./pages/NotificationDeliveryPage'));
const AuditLogPage = React.lazy(() => import('./pages/AuditLogPage'));
const CustomRolesPage = React.lazy(() => import('./pages/CustomRolesPage'));
const OneRosterPage = React.lazy(() => import('./pages/OneRosterPage'));
const DocViewerPage = React.lazy(() => import('./pages/DocViewerPage'));
const LoginPageSSO = React.lazy(() => import('./pages/LoginPageSSO'));
const QuestionBanksPage = React.lazy(() => import('./pages/QuestionBanksPage'));
const ItemBankManagerPage = React.lazy(() => import('./pages/ItemBankManagerPage'));
const StimulusEditorPage = React.lazy(() => import('./pages/StimulusEditorPage'));
const ItemAnalysisPage = React.lazy(() => import('./pages/ItemAnalysisPage'));
const AccommodationsPage = React.lazy(() => import('./pages/AccommodationsPage'));
const AttendancePage = React.lazy(() => import('./pages/AttendancePage'));
import ParentalConsentPage from './pages/ParentalConsentPage';
const PortfoliosPage = React.lazy(() => import('./pages/PortfoliosPage'));
const PortfolioEditorPage = React.lazy(() => import('./pages/PortfolioEditorPage'));
const PortfolioPublicPage = React.lazy(() => import('./pages/PortfolioPublicPage'));
const FERPAPage = React.lazy(() => import('./pages/FERPAPage'));
const GamificationCurrenciesPage = React.lazy(() => import('./pages/GamificationCurrenciesPage'));
const GamificationBadgesPage = React.lazy(() => import('./pages/GamificationBadgesPage'));
const GamificationRecipesPage = React.lazy(() => import('./pages/GamificationRecipesPage'));
const AdminGamificationSettingsPage = React.lazy(() => import('./pages/AdminGamificationSettingsPage'));
const MyBadgesPage = React.lazy(() => import('./pages/MyBadgesPage'));
const ObserverDashboardPage = React.lazy(() => import('./pages/ObserverDashboardPage'));
const AdminHomePage = React.lazy(() => import('./pages/AdminHomePage'));
const AdminCoursesPage = React.lazy(() => import('./pages/AdminCoursesPage'));
const AdminPeoplePage = React.lazy(() => import('./pages/AdminPeoplePage'));
const AdminSettingsPage = React.lazy(() => import('./pages/AdminSettingsPage'));
const SuperAdminSettingsPage = React.lazy(() => import('./pages/SuperAdminSettingsPage'));
import PublicPageView from './pages/PublicPageView';
import NotFoundPage from './pages/NotFoundPage';
const CourseSettingsPage = React.lazy(() => import('./pages/CourseSettingsPage'));
import ProtectedRoute from './components/ProtectedRoute';
import SuperAdminGate from './components/SuperAdminGate';
import { CourseUIProvider } from './contexts/CourseUIContext';
import { useAuth } from './contexts/AuthContext';
import { api } from './services/api';
const SetupWizardPage = React.lazy(() => import('./pages/SetupWizardPage'));
// P3 Features
const FeatureFlagsPage = React.lazy(() => import('./pages/FeatureFlagsPage'));
const MasteryPathsEditorPage = React.lazy(() => import('./pages/MasteryPathsEditorPage'));
const AppointmentGroupsPage = React.lazy(() => import('./pages/AppointmentGroupsPage'));
const AppointmentGroupEditorPage = React.lazy(() => import('./pages/AppointmentGroupEditorPage'));
const OutcomeProficiencyPage = React.lazy(() => import('./pages/OutcomeProficiencyPage'));
const MasteryGradebookPage = React.lazy(() => import('./pages/MasteryGradebookPage'));
const ReadingPreferencesPage = React.lazy(() => import('./pages/ReadingPreferencesPage'));
const GamificationPreferencesPage = React.lazy(() => import('./pages/GamificationPreferencesPage'));
const LanguagePreferencesPage = React.lazy(() => import('./pages/LanguagePreferencesPage'));
const CourseLeaderboardPage = React.lazy(() => import('./pages/CourseLeaderboardPage'));
// Phase 9-B — TOTP MFA enrollment + step-up.
const MFAEnrollPage = React.lazy(() => import('./pages/MFAEnrollPage'));
const MFAVerifyPage = React.lazy(() => import('./pages/MFAVerifyPage'));
// Wave 1.6 follow-up — SIS / OneRoster-provisioned learner sets a
// real password before getting a session.
const PasswordResetRequiredPage = React.lazy(() => import('./pages/PasswordResetRequiredPage'));
// Phase 10-B — passkey enrollment + management.
const PasskeyEnrollPage = React.lazy(() => import('./pages/PasskeyEnrollPage'));
const PasskeyListPage = React.lazy(() => import('./pages/PasskeyListPage'));
const AccountSettingsPage = React.lazy(() => import('./pages/AccountSettingsPage'));
// Smart Search, Commons
const SmartSearchPage = React.lazy(() => import('./pages/SmartSearchPage'));
const CommonsPage = React.lazy(() => import('./pages/CommonsPage'));
const CommonsPublishPage = React.lazy(() => import('./pages/CommonsPublishPage'));

const App = () => {
  const { user, loading } = useAuth();
  const [setupComplete, setSetupComplete] = useState(null); // null = loading

  useEffect(() => {
    api.getSetupStatus()
      .then(({ data }) => setSetupComplete(data.setup_complete))
      .catch(() => setSetupComplete(true)); // If the endpoint fails, assume setup is done
  }, []);

  if (loading || setupComplete === null) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <svg className="animate-spin h-8 w-8 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>
    );
  }

  if (!setupComplete) {
    return (
      <BrowserRouter>
        <React.Suspense fallback={<div className="min-h-screen bg-gray-50 flex items-center justify-center"><svg className="animate-spin h-8 w-8 text-blue-600" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" /></svg></div>}>
          <Routes>
            <Route path="*" element={<SetupWizardPage onSetupComplete={() => setSetupComplete(true)} />} />
          </Routes>
        </React.Suspense>
      </BrowserRouter>
    );
  }

  return (
    <BrowserRouter>
      <React.Suspense fallback={<div className="min-h-screen bg-gray-50 flex items-center justify-center"><div className="text-gray-600">Loading...</div></div>}>
      <Routes>
        <Route path="/login" element={user ? <Navigate to="/" replace /> : <LoginPageSSO />} />
        {/* Phase 9-B — TOTP MFA. Verify route is unauthenticated
            (pending_token IS the credential); enroll route is gated
            via Protected because the user must have a regular session. */}
        <Route path="/mfa/verify" element={<MFAVerifyPage />} />
        <Route path="/mfa/enroll" element={<ProtectedRoute><MFAEnrollPage /></ProtectedRoute>} />
        {/* Wave 1.6 follow-up — anonymous (pending-password-reset
            JWT is the credential). Mirrors /mfa/verify in spirit. */}
        <Route path="/auth/password-set" element={<PasswordResetRequiredPage />} />
        <Route path="/users/self/passkeys" element={<ProtectedRoute><PasskeyListPage /></ProtectedRoute>} />
        <Route path="/users/self/passkeys/enroll" element={<ProtectedRoute><PasskeyEnrollPage /></ProtectedRoute>} />
        <Route path="/consent/verify/:token" element={<ParentalConsentPage />} />
        <Route path="/portfolios/public/:slug" element={<PortfolioPublicPage />} />
        <Route path="/courses/:courseId/p/:slug" element={<PublicPageView />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <DashboardPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/courses"
          element={
            <ProtectedRoute>
              <CoursesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/courses/:courseId"
          element={
            <ProtectedRoute>
              <CourseUIProvider><Outlet /></CourseUIProvider>
            </ProtectedRoute>
          }
        >
          <Route index element={<CoursePage />} />
          <Route path="settings" element={<CourseSettingsPage />} />
          <Route path="assignments" element={<AssignmentsPage />} />
          <Route path="assignments/:assignmentId" element={<AssignmentPage />} />
          <Route path="assignments/:assignmentId/overrides" element={<AssignmentOverridesPage />} />
          <Route path="assignments/:assignmentId/speedgrader" element={<SpeedGraderPage />} />
          <Route path="assignments/:assignmentId/submissions/:userId/docviewer" element={<DocViewerPage />} />
          <Route path="gradebook" element={<GradebookPage />} />
          <Route path="grades" element={<StudentGradesPage />} />
          <Route path="modules" element={<ModulesPage />} />
          <Route path="people" element={<PeoplePage />} />
          <Route path="pages" element={<PagesPage />} />
          <Route path="pages/:slug" element={<PageDetailPage />} />
          <Route path="quizzes" element={<QuizzesPage />} />
          <Route path="quizzes/:quizId/take" element={<QuizTakePage />} />
          <Route path="quizzes/:quizId/edit" element={<QuizEditorPage />} />
          <Route path="quizzes/:quizId/submissions/:submissionId/review" element={<QuizReviewPage />} />
          <Route path="quizzes/:quizId/submissions" element={<QuizSubmissionsPage />} />
          <Route path="quizzes/:quizId/statistics" element={<QuizStatisticsPage />} />
          <Route path="quizzes/:quizId/item-analysis" element={<ItemAnalysisPage />} />
          <Route path="item-banks" element={<ItemBankManagerPage />} />
          <Route path="stimuli" element={<StimulusEditorPage />} />
          <Route path="stimuli/:stimulusId" element={<StimulusEditorPage />} />
          <Route path="discussions" element={<DiscussionsPage />} />
          <Route path="discussions/:topicId" element={<DiscussionTopicPage />} />
          <Route path="files" element={<FilesPage />} />
          <Route path="external_tools" element={<ExternalToolsPage />} />
          <Route path="rubrics" element={<RubricsPage />} />
          <Route path="calendar" element={<CalendarPage />} />
          <Route path="announcements" element={<AnnouncementsPage />} />
          <Route path="syllabus" element={<SyllabusPage />} />
          <Route path="outcomes" element={<LearningOutcomesPage />} />
          <Route path="groups" element={<GroupsPage />} />
          <Route path="blueprint" element={<BlueprintPage />} />
          <Route path="pacing" element={<CoursePacingPage />} />
          <Route path="collaborations" element={<CollaborationsPage />} />
          <Route path="conferences" element={<ConferencesPage />} />
          <Route path="analytics" element={<AnalyticsPage />} />
          <Route path="audit_log" element={<AuditLogPage />} />
          <Route path="gamification/currencies" element={<GamificationCurrenciesPage />} />
          <Route path="gamification/badges" element={<GamificationBadgesPage />} />
          <Route path="gamification/recipes" element={<GamificationRecipesPage />} />
          <Route path="accommodations" element={<AccommodationsPage />} />
          <Route path="attendance" element={<AttendancePage />} />
          <Route path="question_banks" element={<QuestionBanksPage />} />
          <Route path="content_import" element={<ContentImportPage />} />
          {/* P3 Features */}
          <Route path="assignments/:assignmentId/mastery_paths" element={<MasteryPathsEditorPage />} />
          <Route path="appointment_groups" element={<AppointmentGroupsPage />} />
          <Route path="appointment_groups/new" element={<AppointmentGroupEditorPage />} />
          <Route path="appointment_groups/:groupId/edit" element={<AppointmentGroupEditorPage />} />
          <Route path="outcomes/proficiency" element={<OutcomeProficiencyPage />} />
          <Route path="mastery_gradebook" element={<MasteryGradebookPage />} />
          
          <Route path="smart_search" element={<SmartSearchPage />} />
          <Route path="commons/publish" element={<CommonsPublishPage />} />
        </Route>
        <Route
          path="/commons"
          element={
            <ProtectedRoute>
              <CommonsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/settings/tokens"
          element={
            <ProtectedRoute>
              <AccessTokensPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/developer_keys"
          element={
            <ProtectedRoute>
              <DeveloperKeysPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/sis_import"
          element={
            <ProtectedRoute>
              <SISImportPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/grading_periods"
          element={
            <ProtectedRoute>
              <GradingPeriodsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/calendar"
          element={
            <ProtectedRoute>
              <CalendarPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/inbox"
          element={
            <ProtectedRoute>
              <InboxPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/notifications"
          element={
            <ProtectedRoute>
              <NotificationsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/settings/notifications"
          element={
            <ProtectedRoute>
              <NotificationPreferencesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/settings/language"
          element={
            <ProtectedRoute>
              <LanguagePreferencesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/graphiql"
          element={
            <ProtectedRoute>
              <GraphiQLPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/auth_providers"
          element={
            <ProtectedRoute>
              <AuthProvidersPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/terms"
          element={
            <ProtectedRoute>
              <EnrollmentTermsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/roles"
          element={
            <ProtectedRoute>
              <CustomRolesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/oneroster"
          element={
            <ProtectedRoute>
              <OneRosterPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/settings/notification_deliveries"
          element={
            <ProtectedRoute>
              <NotificationDeliveryPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/portfolios"
          element={
            <ProtectedRoute>
              <PortfoliosPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/portfolios/:portfolioId/edit"
          element={
            <ProtectedRoute>
              <PortfolioEditorPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin"
          element={
            <ProtectedRoute>
              <AdminHomePage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/courses"
          element={
            <ProtectedRoute>
              <AdminCoursesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/people"
          element={
            <ProtectedRoute>
              <AdminPeoplePage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/settings"
          element={
            <ProtectedRoute>
              <AdminSettingsPage />
            </ProtectedRoute>
          }
        />
        {/*
          Super-Admin Settings Engine. ProtectedRoute (session check)
          + SuperAdminGate (role==='super_admin' check) are layered;
          the server's RequireSuperAdmin middleware is the
          authoritative gate either way. UI gate is for UX only —
          users without the role land on a clean explainer panel
          instead of a half-rendered form that 403s on every call.
        */}
        <Route
          path="/superadmin/settings"
          element={
            <ProtectedRoute>
              <SuperAdminGate>
                <SuperAdminSettingsPage />
              </SuperAdminGate>
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/ferpa"
          element={
            <ProtectedRoute>
              <FERPAPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/gamification/currencies"
          element={
            <ProtectedRoute>
              <GamificationCurrenciesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/gamification/badges"
          element={
            <ProtectedRoute>
              <GamificationBadgesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/gamification/recipes"
          element={
            <ProtectedRoute>
              <GamificationRecipesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/gamification/settings"
          element={
            <ProtectedRoute>
              <AdminGamificationSettingsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/courses/:courseId/gamification/recipes"
          element={
            <ProtectedRoute>
              <GamificationRecipesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/courses/:courseId/leaderboard"
          element={
            <ProtectedRoute>
              <CourseLeaderboardPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile/badges"
          element={
            <ProtectedRoute>
              <MyBadgesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/observer"
          element={
            <ProtectedRoute>
              <ObserverDashboardPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile/reading-preferences"
          element={
            <ProtectedRoute>
              <ReadingPreferencesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile/gamification"
          element={
            <ProtectedRoute>
              <GamificationPreferencesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/profile/settings"
          element={
            <ProtectedRoute>
              <AccountSettingsPage />
            </ProtectedRoute>
          }
        />
        {/* P3 Features — admin scope */}
        <Route
          path="/admin/feature_flags"
          element={
            <ProtectedRoute>
              <FeatureFlagsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/admin/outcome_proficiency"
          element={
            <ProtectedRoute>
              <OutcomeProficiencyPage />
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      </React.Suspense>
    </BrowserRouter>
  );
};

export default App;
