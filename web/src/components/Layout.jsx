import React, { useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  LayoutDashboard, BookOpen, Calendar, Mail, LogOut,
  Briefcase, Eye, Settings, Home, Inbox, Menu, X, AlertTriangle, Library, Shield
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../contexts/AuthContext';
import { useCourseUI } from '../contexts/CourseUIContext';
import { api } from '../services/api';
import SkipToContent from './SkipToContent';
import InstallPrompt from './InstallPrompt';
import OfflineIndicator from './OfflineIndicator';
import AdminNav from './AdminNav';
import NotificationBell from './NotificationBell';
import MobileBottomNav from './MobileBottomNav';
import ThemeToggle from './ThemeToggle';
import LanguageSwitcher from './LanguageSwitcher';
import CurrencyPills from './gamification/CurrencyPills';
import BrandLogo from './brand/BrandLogo';
import UserMenu from './UserMenu';

// Nav items are built inside the component so labels translate when the
// active language changes (useTranslation only runs in the render tree).
const buildBaseNav = (t) => [
  { to: '/', icon: LayoutDashboard, label: t('nav.dashboard') },
  { to: '/courses', icon: BookOpen, label: t('nav.courses') },
  { to: '/calendar', icon: Calendar, label: t('nav.calendar') },
  { to: '/inbox', icon: Mail, label: t('nav.inbox') },
  { to: '/portfolios', icon: Briefcase, label: t('nav.portfolios') },
  { to: '/commons', icon: Library, label: t('nav.commons') },
];

const buildAdminLeadNav = (t) => [
  { to: '/admin', icon: Shield, label: t('nav.admin') },
];

const buildAdminTrailNav = (t) => [
  { to: '/observer', icon: Eye, label: t('nav.observer') },
];

const buildSimplifiedNav = (t) => [
  { to: '/', icon: Home, label: t('nav.home') },
  { to: '/', icon: LayoutDashboard, label: t('nav.dashboard') },
  { to: '/inbox', icon: Inbox, label: t('nav.inbox') },
];

const NavItem = ({ to, icon: Icon, label, active }) => (
  <Link
    to={to}
    className={`relative group flex items-center justify-center w-10 h-10 rounded-md transition-colors
      ${active
        ? 'bg-white/15 text-white'
        : 'text-gray-300 hover:bg-white/10 hover:text-white'
      }
    `}
  >
    <Icon className="w-5 h-5" />
    <span className="absolute left-full ml-2 px-2 py-1 rounded bg-chrome-tooltip text-chrome-tooltip-fg text-xs font-medium whitespace-nowrap opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-50">
      {label}
    </span>
  </Link>
);

const SimplifiedNavItem = ({ to, icon: Icon, label, active }) => (
  <Link
    to={to}
    className={`flex flex-col items-center justify-center w-20 py-2 rounded-md transition-colors
      ${active
        ? 'bg-white/15 text-white'
        : 'text-gray-300 hover:bg-white/10 hover:text-white'
      }
    `}
  >
    <Icon className="w-7 h-7" />
    <span className="text-xs mt-1">{label}</span>
  </Link>
);

const isAdminRoute = (pathname) =>
  pathname === '/admin' ||
  pathname.startsWith('/admin/') ||
  pathname.startsWith('/settings/') ||
  pathname === '/graphiql';

// Masquerade banner shown when an admin is acting as another user
const MasqueradeBanner = ({ userName, onStopMasquerade, stopping }) => {
  const { t } = useTranslation();
  return (
    <div
      className="fixed top-0 left-0 right-0 z-[60] bg-accent-warning text-white px-4 py-2 flex items-center justify-center gap-3 shadow-md"
      role="alert"
      aria-live="polite"
    >
      <AlertTriangle className="w-4 h-4 flex-shrink-0" />
      <span className="text-sm font-medium">
        {t('nav.actingAsPrefix')} <strong>{userName}</strong>
      </span>
      <button
        onClick={onStopMasquerade}
        disabled={stopping}
        className="ml-2 px-3 py-1 text-xs font-semibold bg-black/20 text-white rounded hover:bg-black/30 disabled:opacity-50 transition-colors"
      >
        {stopping ? t('nav.restoring') : t('nav.stopMasquerading')}
      </button>
    </div>
  );
};

// Preview banner shown when a staff user is opt-in previewing a course's K-2 / 3-5 layout.
const PreviewBanner = ({ mode, onExit, offset }) => {
  const { t } = useTranslation();
  const modeLabel = mode === 'k2' ? t('nav.modeK2') : t('nav.mode35');
  return (
    <div
      className="fixed left-0 right-0 z-[60] bg-brand-600 text-white px-4 py-2 flex items-center justify-center gap-3 shadow-md"
      style={{ top: offset }}
      role="status"
      aria-live="polite"
    >
      <Eye className="w-4 h-4 flex-shrink-0" />
      <span className="text-sm font-medium">
        {t('nav.previewingStudentView', { mode: modeLabel })}
      </span>
      <button
        onClick={onExit}
        className="ml-2 px-3 py-1 text-xs font-semibold bg-black/20 text-white rounded hover:bg-black/30 transition-colors"
      >
        {t('nav.exitPreview')}
      </button>
    </div>
  );
};

const Layout = ({ children }) => {
  const { t } = useTranslation();
  const { user, logout, refreshUser } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const showAdminNav = isAdminRoute(location.pathname);
  const { isK2, is35, effectiveMode, isPreview, exitPreview } = useCourseUI();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [stoppingMasquerade, setStoppingMasquerade] = useState(false);
  const isAdmin = user?.role === 'admin';
  const isMasquerading = !!user?.masquerading_as;
  const baseNav = buildBaseNav(t);
  const adminLeadNav = buildAdminLeadNav(t);
  const adminTrailNav = buildAdminTrailNav(t);
  const simplifiedNav = buildSimplifiedNav(t);
  const primaryNav = isAdmin ? [...adminLeadNav, ...baseNav, ...adminTrailNav] : baseNav;

  // Banner stack: masquerade goes on top (40px), preview sits below it.
  const bannerCount = (isMasquerading ? 1 : 0) + (isPreview ? 1 : 0);
  const previewBannerTop = isMasquerading ? 40 : 0;
  const topPaddingPx = bannerCount * 40;
  const topPaddingStyle = topPaddingPx ? { paddingTop: topPaddingPx } : undefined;
  const sidebarTopOffset = topPaddingPx ? { top: topPaddingPx } : undefined;

  const handleLogout = async () => {
    await logout();
    window.location.href = '/login';
  };

  const handleStopMasquerade = async () => {
    setStoppingMasquerade(true);
    try {
      await api.endMasquerade();
      await refreshUser();
      navigate('/');
    } catch (err) {
      console.error('Failed to stop masquerading:', err);
    } finally {
      setStoppingMasquerade(false);
    }
  };

  const isActive = (path) => {
    if (path === '/') return location.pathname === '/';
    if (path === '/admin') return isAdminRoute(location.pathname);
    return location.pathname.startsWith(path);
  };

  // K-2 mode: hide sidebar entirely (only renders for actual K-2 students or staff in preview)
  if (isK2) {
    return (
      <div className="min-h-screen bg-sky-50" style={topPaddingStyle}>
        {isMasquerading && (
          <MasqueradeBanner
            userName={user.masquerading_as}
            onStopMasquerade={handleStopMasquerade}
            stopping={stoppingMasquerade}
          />
        )}
        {isPreview && (
          <PreviewBanner mode={effectiveMode} onExit={exitPreview} offset={previewBannerTop} />
        )}
        <OfflineIndicator />
        <SkipToContent />
        <div className="flex-1">
          <main id="main-content" className="max-w-7xl mx-auto px-6 py-8 pb-16 md:pb-0" role="main">
            {children}
          </main>
        </div>
        <MobileBottomNav />
        <InstallPrompt />
      </div>
    );
  }

  // 3-5 mode: simplified sidebar with larger icons and text labels
  if (is35) {
    return (
      <div className="min-h-screen bg-surface-1 flex" style={topPaddingStyle}>
        {isMasquerading && (
          <MasqueradeBanner
            userName={user.masquerading_as}
            onStopMasquerade={handleStopMasquerade}
            stopping={stoppingMasquerade}
          />
        )}
        {isPreview && (
          <PreviewBanner mode={effectiveMode} onExit={exitPreview} offset={previewBannerTop} />
        )}
        <OfflineIndicator />
        <SkipToContent />

        <aside
          className="fixed inset-y-0 left-0 z-30 flex flex-col items-center w-20 bg-chrome-sidebar"
          style={sidebarTopOffset}
          role="navigation"
          aria-label={t('nav.globalNavigation')}
        >
          <div className="flex items-center justify-center h-14 border-b border-white/10 w-full">
            <Link to="/" className="text-white" title={t('nav.paperLms')}>
              <BrandLogo size={32} />
            </Link>
          </div>

          <nav className="flex-1 overflow-y-auto overflow-x-hidden sidebar-scroll py-3 space-y-1 flex flex-col items-center">
            {simplifiedNav.map((item) => (
              <SimplifiedNavItem key={item.to + item.label} {...item} active={isActive(item.to)} />
            ))}
          </nav>

          <div className="border-t border-white/10 py-2 flex flex-col items-center w-full">
            <button
              onClick={handleLogout}
              title={t('nav.logout')}
              className="flex flex-col items-center justify-center w-20 py-2 rounded-md text-gray-300 hover:bg-white/10 hover:text-white transition-colors"
            >
              <LogOut className="w-7 h-7" />
              <span className="text-xs mt-1">{t('nav.logout')}</span>
            </button>
          </div>
        </aside>

        {showAdminNav && <AdminNav />}

        <div className={`flex-1 ${showAdminNav ? 'ml-[284px]' : 'ml-20'}`}>
          <CurrencyPills />
          <main id="main-content" className="max-w-7xl mx-auto px-6 py-8 pb-16 md:pb-0" role="main">
            {children}
          </main>
        </div>

        <MobileBottomNav />
        <InstallPrompt />
      </div>
    );
  }

  // Standard mode
  const sidebarContent = (
    <>
      {/* Logo */}
      <div className="flex items-center justify-center h-14 border-b border-white/10 w-full">
        <Link to="/" className="text-white relative group" title={t('nav.paperLms')} onClick={() => setMobileMenuOpen(false)}>
          <BrandLogo size={28} />
          <span className="absolute left-full ml-2 px-2 py-1 rounded bg-chrome-tooltip text-chrome-tooltip-fg text-xs font-medium whitespace-nowrap opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-50 hidden md:block">
            {t('nav.paperLms')}
          </span>
        </Link>
      </div>

      {/* Primary nav */}
      <nav className="flex-1 overflow-y-auto overflow-x-hidden sidebar-scroll py-3 space-y-1 flex flex-col items-center">
        {primaryNav.map((item) => (
          <Link
            key={item.to}
            to={item.to}
            onClick={() => setMobileMenuOpen(false)}
            className={`relative group flex items-center justify-center w-10 h-10 rounded-md transition-colors
              ${isActive(item.to)
                ? 'bg-white/15 text-white'
                : 'text-gray-300 hover:bg-white/10 hover:text-white'
              }
            `}
          >
            <item.icon className="w-5 h-5" />
            <span className="absolute left-full ml-2 px-2 py-1 rounded bg-chrome-tooltip text-chrome-tooltip-fg text-xs font-medium whitespace-nowrap opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-50">
              {item.label}
            </span>
          </Link>
        ))}
      </nav>

      {/* User section at bottom */}
      <div className="border-t border-white/10 py-2 space-y-1 flex flex-col items-center w-full">
        <NotificationBell />
        <ThemeToggle />
        <LanguageSwitcher />
        <UserMenu
          user={user}
          onLogout={() => { setMobileMenuOpen(false); handleLogout(); }}
        />
      </div>
    </>
  );

  return (
    <div className="min-h-screen bg-surface-1 flex" style={topPaddingStyle}>
      {isMasquerading && (
        <MasqueradeBanner
          userName={user.masquerading_as}
          onStopMasquerade={handleStopMasquerade}
          stopping={stoppingMasquerade}
        />
      )}
      {isPreview && (
        <PreviewBanner mode={effectiveMode} onExit={exitPreview} offset={previewBannerTop} />
      )}
      <OfflineIndicator />
      <SkipToContent />

      {/* Mobile hamburger button */}
      <button
        onClick={() => setMobileMenuOpen(true)}
        className="fixed left-3 z-40 md:hidden p-2 rounded-md bg-chrome-sidebar text-white shadow-lg"
        style={{ top: topPaddingPx ? topPaddingPx + 12 : 12 }}
        aria-label={t('nav.openMenu')}
      >
        <Menu className="w-5 h-5" />
      </button>

      {/* Mobile sidebar overlay */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="fixed inset-0 bg-black/50" onClick={() => setMobileMenuOpen(false)} />
          <aside
            className="fixed inset-y-0 left-0 z-50 flex flex-col items-center w-16 bg-chrome-sidebar"
            style={sidebarTopOffset}
            role="navigation"
            aria-label={t('nav.globalNavigation')}
          >
            {sidebarContent}
          </aside>
          <button
            onClick={() => setMobileMenuOpen(false)}
            className="fixed left-[72px] z-50 p-1 rounded-full bg-surface-0 text-text-secondary shadow"
            style={{ top: topPaddingPx ? topPaddingPx + 12 : 12 }}
            aria-label={t('nav.closeMenu')}
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Desktop sidebar */}
      <aside
        className="hidden md:flex fixed inset-y-0 left-0 z-30 flex-col items-center w-16 bg-chrome-sidebar"
        style={sidebarTopOffset}
        role="navigation"
        aria-label={t('nav.globalNavigation')}
      >
        {sidebarContent}
      </aside>

      {/* Admin sub-nav panel */}
      {showAdminNav && <AdminNav />}

      {/* Main content area */}
      <div className={`flex-1 ${showAdminNav ? 'md:ml-[280px] ml-0' : 'md:ml-16 ml-0'}`}>
        <div className="hidden md:block">
          <CurrencyPills />
        </div>
        <main id="main-content" className="max-w-7xl mx-auto px-6 py-8 pb-16 md:pb-0 pt-14 md:pt-8" role="main">
          {children}
        </main>
      </div>

      <MobileBottomNav />
      <InstallPrompt />
    </div>
  );
};

export default Layout;
