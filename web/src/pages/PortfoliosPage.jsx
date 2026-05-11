import React, { useState, useEffect, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  Plus,
  Eye,
  EyeOff,
  Pencil,
  Trash2,
  Download,
  ExternalLink,
  Search,
  Filter,
  Clock,
  BarChart3,
  Sparkles,
  Briefcase,
  X,
  ChevronDown,
  Globe,
  FileText,
  Palette,
  MoreVertical,
  Archive,
  Copy,
  Star,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

/* ───────── Theme meta (shared with editor) ───────── */
const THEME_META = {
  clean_modern:       { label: 'Clean Modern',       color: 'bg-brand-500',   accent: 'text-brand-600',  ring: 'ring-blue-300' },
  creative_bold:      { label: 'Creative Bold',      color: 'bg-purple-600', accent: 'text-purple-600', ring: 'ring-purple-300' },
  academic_classic:   { label: 'Academic Classic',    color: 'bg-amber-600',  accent: 'text-accent-warning', ring: 'ring-amber-300' },
  minimal_dark:       { label: 'Minimal Dark',        color: 'bg-gray-800',   accent: 'text-text-disabled',  ring: 'ring-gray-500' },
  developer_portfolio:{ label: 'Developer Portfolio', color: 'bg-emerald-600', accent: 'text-accent-success', ring: 'ring-emerald-300' },
};

const STATUS_OPTIONS = [
  { value: 'all',       label: 'All Portfolios' },
  { value: 'draft',     label: 'Drafts' },
  { value: 'published', label: 'Published' },
  { value: 'archived',  label: 'Archived' },
];

const TEMPLATES = [
  { id: 'blank',      name: 'Blank Portfolio',    description: 'Start from scratch with a clean slate', icon: FileText },
  { id: 'student',    name: 'Student Showcase',   description: 'Pre-built sections for coursework, projects, and skills', icon: Star },
  { id: 'creative',   name: 'Creative Portfolio', description: 'Gallery-focused layout for visual work', icon: Palette },
  { id: 'career',     name: 'Career Ready',       description: 'Professional layout with experience timeline', icon: Briefcase },
];

/* ────────── helpers ────────── */
const formatDate = (dateStr) => {
  if (!dateStr) return 'Never';
  const d = new Date(dateStr);
  const now = new Date();
  const diffMs = now - d;
  const diffMins = Math.floor(diffMs / 60000);
  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHrs = Math.floor(diffMins / 60);
  if (diffHrs < 24) return `${diffHrs}h ago`;
  const diffDays = Math.floor(diffHrs / 24);
  if (diffDays < 7) return `${diffDays}d ago`;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
};

const formatViewCount = (n) => {
  if (!n || n === 0) return '0 views';
  if (n === 1) return '1 view';
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k views`;
  return `${n} views`;
};

/* ═══════════════════════════════════════════════════════
   Component: PortfoliosPage
   ═══════════════════════════════════════════════════════ */
const PortfoliosPage = () => {
  const { user } = useAuth();
  const navigate = useNavigate();

  /* ── state ── */
  const [portfolios, setPortfolios] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [statusFilter, setStatusFilter] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createTemplate, setCreateTemplate] = useState('blank');
  const [createTheme, setCreateTheme] = useState('clean_modern');
  const [creating, setCreating] = useState(false);
  const [openMenuId, setOpenMenuId] = useState(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState(null);

  /* ── fetch ── */
  const fetchPortfolios = useCallback(async () => {
    try {
      const { data } = await api.listPortfolios();
      setPortfolios(data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [user?.id]);

  useEffect(() => {
    if (user?.id) fetchPortfolios();
  }, [user?.id, fetchPortfolios]);

  /* ── actions ── */
  const handleCreate = async (e) => {
    e.preventDefault();
    if (!createName.trim()) return;
    setCreating(true);
    try {
      const portfolio = await api.createPortfolio({
        title: createName.trim(),
        template: createTemplate,
        theme: createTheme,
      });
      setShowCreateModal(false);
      setCreateName('');
      setCreateTemplate('blank');
      setCreateTheme('clean_modern');
      navigate(`/portfolios/${portfolio.id}/edit`);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handlePublishToggle = async (portfolio) => {
    try {
      if (portfolio.status === 'published') {
        await api.unpublishPortfolio(portfolio.id);
      } else {
        await api.publishPortfolio(portfolio.id);
      }
      await fetchPortfolios();
    } catch (err) {
      setError(err.message);
    }
    setOpenMenuId(null);
  };

  const handleArchive = async (portfolio) => {
    try {
      await api.updatePortfolio(portfolio.id, { status: 'archived' });
      await fetchPortfolios();
    } catch (err) {
      setError(err.message);
    }
    setOpenMenuId(null);
  };

  const handleDuplicate = async (portfolio) => {
    try {
      await api.duplicatePortfolio(portfolio.id);
      await fetchPortfolios();
    } catch (err) {
      setError(err.message);
    }
    setOpenMenuId(null);
  };

  const handleDelete = async (id) => {
    try {
      await api.deletePortfolio(id);
      setDeleteConfirmId(null);
      await fetchPortfolios();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleExport = async (portfolio, format) => {
    try {
      if (format === 'html') {
        const response = await api.exportPortfolioHTML(portfolio.id);
        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `${portfolio.title || 'portfolio'}.zip`;
        a.click();
        URL.revokeObjectURL(url);
      } else {
        const response = await api.exportPortfolioPDF(portfolio.id);
        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `${portfolio.title || 'portfolio'}.pdf`;
        a.click();
        URL.revokeObjectURL(url);
      }
    } catch (err) {
      setError(err.message);
    }
    setOpenMenuId(null);
  };

  /* ── filter + search ── */
  const filtered = portfolios.filter((p) => {
    if (statusFilter !== 'all' && p.status !== statusFilter) return false;
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      return (
        (p.title || '').toLowerCase().includes(q) ||
        (p.tagline || '').toLowerCase().includes(q)
      );
    }
    return true;
  });

  /* ── close menu on outside click ── */
  useEffect(() => {
    const handleClick = () => setOpenMenuId(null);
    if (openMenuId !== null) {
      document.addEventListener('click', handleClick);
      return () => document.removeEventListener('click', handleClick);
    }
  }, [openMenuId]);

  /* ════════════════ Render ════════════════ */

  /* Loading */
  if (loading) {
    return (
      <Layout>
        <div className="flex flex-col items-center justify-center py-24" role="status" aria-label="Loading portfolios">
          <div className="relative w-16 h-16 mb-6">
            <div className="absolute inset-0 rounded-full border-4 border-blue-100" />
            <div className="absolute inset-0 rounded-full border-4 border-brand-500 border-t-transparent animate-spin" />
          </div>
          <p className="text-text-tertiary text-lg font-medium">Loading your portfolios...</p>
        </div>
      </Layout>
    );
  }

  /* Error */
  if (error && portfolios.length === 0) {
    return (
      <Layout>
        <div className="text-center py-16">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-accent-danger/10 mb-4">
            <X className="w-8 h-8 text-accent-danger" />
          </div>
          <h2 className="text-xl font-semibold text-text-primary mb-2">Something went wrong</h2>
          <p className="text-text-tertiary mb-6">{error}</p>
          <button
            onClick={() => { setError(null); setLoading(true); fetchPortfolios(); }}
            className="px-5 py-2.5 bg-brand-600 text-white rounded-lg hover:bg-brand-700 transition-colors font-medium"
          >
            Try Again
          </button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      {/* ── Page Header ── */}
      <section className="mb-8" aria-labelledby="portfolios-heading">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div>
            <h1 id="portfolios-heading" className="text-3xl font-bold text-text-primary tracking-tight">
              ePortfolios
            </h1>
            <p className="text-text-tertiary mt-1">
              Showcase your best work, reflect on your learning, and share with the world.
            </p>
          </div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl hover:from-blue-700 hover:to-indigo-700 transition-all shadow-lg shadow-blue-500/25 font-medium text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
            aria-label="Create a new portfolio"
          >
            <Plus className="w-4 h-4" aria-hidden="true" />
            Create Portfolio
          </button>
        </div>

        {/* ── Filters ── */}
        {portfolios.length > 0 && (
          <div className="mt-6 flex flex-col sm:flex-row gap-3">
            {/* Search */}
            <div className="relative flex-1 max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-disabled" aria-hidden="true" />
              <input
                type="search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search portfolios..."
                className="w-full pl-10 pr-4 py-2.5 border border-border-default rounded-xl text-sm bg-surface-0 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-shadow"
                aria-label="Search portfolios"
              />
            </div>
            {/* Status filter */}
            <div className="flex gap-1 p-1 bg-surface-2 rounded-xl" role="tablist" aria-label="Filter portfolios by status">
              {STATUS_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => setStatusFilter(opt.value)}
                  role="tab"
                  aria-selected={statusFilter === opt.value}
                  className={`px-4 py-2 text-sm font-medium rounded-lg transition-all ${
                    statusFilter === opt.value
                      ? 'bg-surface-0 text-text-primary shadow-sm'
                      : 'text-text-tertiary hover:text-text-secondary'
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </section>

      {/* ── Inline error banner ── */}
      {error && portfolios.length > 0 && (
        <div className="mb-6 p-4 bg-accent-danger/10 border border-accent-danger/30 rounded-xl flex items-center gap-3" role="alert">
          <X className="w-5 h-5 text-accent-danger shrink-0" aria-hidden="true" />
          <p className="text-sm text-accent-danger flex-1">{error}</p>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-accent-danger" aria-label="Dismiss error">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* ── Empty State ── */}
      {portfolios.length === 0 ? (
        <div className="text-center py-20 px-6">
          <div className="relative inline-block mb-8">
            <div className="w-28 h-28 bg-gradient-to-br from-blue-50 to-indigo-100 rounded-3xl flex items-center justify-center rotate-3 shadow-sm">
              <Sparkles className="w-14 h-14 text-brand-500" aria-hidden="true" />
            </div>
            <div className="absolute -bottom-2 -right-2 w-10 h-10 bg-gradient-to-br from-purple-400 to-pink-400 rounded-xl flex items-center justify-center -rotate-6 shadow-md">
              <Star className="w-5 h-5 text-white" aria-hidden="true" />
            </div>
          </div>
          <h2 className="text-2xl font-bold text-text-primary mb-3">Your story starts here</h2>
          <p className="text-text-tertiary max-w-lg mx-auto mb-2 text-lg">
            An ePortfolio is more than a collection of work -- it is a living testament to your growth,
            creativity, and unique perspective.
          </p>
          <p className="text-text-disabled max-w-md mx-auto mb-8">
            Curate your best projects, reflect on what you have learned, and share a professional
            portfolio that makes employers and collaborators take notice.
          </p>
          <button
            onClick={() => setShowCreateModal(true)}
            className="inline-flex items-center gap-2 px-6 py-3 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl hover:from-blue-700 hover:to-indigo-700 transition-all shadow-lg shadow-blue-500/25 font-semibold focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
          >
            <Plus className="w-5 h-5" aria-hidden="true" />
            Create Your First Portfolio
          </button>
        </div>
      ) : filtered.length === 0 ? (
        /* No results for current filter */
        <div className="text-center py-16">
          <Filter className="w-12 h-12 text-gray-300 mx-auto mb-4" aria-hidden="true" />
          <h3 className="text-lg font-semibold text-text-secondary mb-1">No portfolios found</h3>
          <p className="text-text-disabled">
            {searchQuery ? 'Try a different search term.' : 'No portfolios match this filter.'}
          </p>
        </div>
      ) : (
        /* ── Portfolio Grid ── */
        <div
          className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6"
          role="list"
          aria-label="Your portfolios"
        >
          {filtered.map((portfolio) => {
            const theme = THEME_META[portfolio.theme] || THEME_META.clean_modern;
            const isPublished = portfolio.status === 'published';
            const isArchived = portfolio.status === 'archived';

            return (
              <article
                key={portfolio.id}
                role="listitem"
                className="group relative bg-surface-0 rounded-2xl shadow-sm border border-border-subtle hover:shadow-xl hover:border-border-default transition-all duration-300 overflow-hidden flex flex-col"
              >
                {/* Card thumbnail / cover */}
                <div className="relative h-40 overflow-hidden">
                  {portfolio.cover_image_url ? (
                    <img
                      src={portfolio.cover_image_url}
                      alt=""
                      className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
                    />
                  ) : (
                    <div className={`w-full h-full ${
                      portfolio.theme === 'minimal_dark'
                        ? 'bg-gradient-to-br from-gray-800 to-gray-900'
                        : portfolio.theme === 'creative_bold'
                        ? 'bg-gradient-to-br from-purple-600 to-pink-500'
                        : portfolio.theme === 'academic_classic'
                        ? 'bg-gradient-to-br from-amber-100 to-orange-100'
                        : portfolio.theme === 'developer_portfolio'
                        ? 'bg-gradient-to-br from-gray-900 to-emerald-900'
                        : 'bg-gradient-to-br from-blue-50 to-indigo-100'
                    } flex items-center justify-center`}>
                      <div className="text-center">
                        <FileText className={`w-10 h-10 mx-auto mb-2 ${
                          portfolio.theme === 'minimal_dark' || portfolio.theme === 'developer_portfolio' || portfolio.theme === 'creative_bold'
                            ? 'text-white/40'
                            : 'text-text-disabled/60'
                        }`} aria-hidden="true" />
                        <span className={`text-xs font-medium ${
                          portfolio.theme === 'minimal_dark' || portfolio.theme === 'developer_portfolio' || portfolio.theme === 'creative_bold'
                            ? 'text-white/30'
                            : 'text-text-disabled/50'
                        }`}>
                          {theme.label}
                        </span>
                      </div>
                    </div>
                  )}

                  {/* Status badge */}
                  <div className="absolute top-3 left-3">
                    <span className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold backdrop-blur-sm ${
                      isPublished
                        ? 'bg-accent-success/90 text-white'
                        : isArchived
                        ? 'bg-gray-500/90 text-white'
                        : 'bg-surface-0/90 text-text-secondary ring-1 ring-gray-200'
                    }`}>
                      {isPublished ? <Globe className="w-3 h-3" aria-hidden="true" /> : isArchived ? <Archive className="w-3 h-3" aria-hidden="true" /> : <Pencil className="w-3 h-3" aria-hidden="true" />}
                      {isPublished ? 'Published' : isArchived ? 'Archived' : 'Draft'}
                    </span>
                  </div>

                  {/* Theme indicator dot */}
                  <div className="absolute top-3 right-3">
                    <span
                      className={`block w-4 h-4 rounded-full ${theme.color} ring-2 ring-white shadow`}
                      title={`Theme: ${theme.label}`}
                      aria-label={`Theme: ${theme.label}`}
                    />
                  </div>

                  {/* Hover overlay */}
                  <div className="absolute inset-0 bg-black/0 group-hover:bg-black/20 transition-colors duration-300 flex items-center justify-center opacity-0 group-hover:opacity-100">
                    <Link
                      to={`/portfolios/${portfolio.id}/edit`}
                      className="px-4 py-2 bg-surface-0 rounded-lg text-sm font-semibold text-text-primary shadow-lg hover:bg-surface-1 transition-colors focus:outline-none focus:ring-2 focus:ring-white"
                      aria-label={`Edit ${portfolio.title}`}
                    >
                      Open Editor
                    </Link>
                  </div>
                </div>

                {/* Card body */}
                <div className="p-5 flex-1 flex flex-col">
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <Link
                      to={`/portfolios/${portfolio.id}/edit`}
                      className="text-lg font-bold text-text-primary hover:text-brand-600 transition-colors line-clamp-1 focus:outline-none focus:underline"
                    >
                      {portfolio.title || 'Untitled Portfolio'}
                    </Link>

                    {/* Kebab menu */}
                    <div className="relative shrink-0">
                      <button
                        onClick={(e) => { e.stopPropagation(); setOpenMenuId(openMenuId === portfolio.id ? null : portfolio.id); }}
                        className="p-1.5 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                        aria-label={`Actions for ${portfolio.title}`}
                        aria-haspopup="true"
                        aria-expanded={openMenuId === portfolio.id}
                      >
                        <MoreVertical className="w-4 h-4" aria-hidden="true" />
                      </button>

                      {openMenuId === portfolio.id && (
                        <div
                          className="absolute right-0 top-full mt-1 w-52 bg-surface-0 rounded-xl shadow-xl border border-border-subtle py-1.5 z-30 animate-in fade-in slide-in-from-top-2"
                          role="menu"
                          aria-label="Portfolio actions"
                        >
                          <Link
                            to={`/portfolios/${portfolio.id}/edit`}
                            className="flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors"
                            role="menuitem"
                          >
                            <Pencil className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Edit
                          </Link>
                          {isPublished && (
                            <Link
                              to={`/p/${portfolio.slug || portfolio.id}`}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="flex items-center gap-2.5 px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors"
                              role="menuitem"
                            >
                              <ExternalLink className="w-4 h-4 text-text-disabled" aria-hidden="true" /> View Public Page
                            </Link>
                          )}
                          <button
                            onClick={() => handlePublishToggle(portfolio)}
                            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                            role="menuitem"
                          >
                            {isPublished ? (
                              <><EyeOff className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Unpublish</>
                            ) : (
                              <><Globe className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Publish</>
                            )}
                          </button>
                          <button
                            onClick={() => handleDuplicate(portfolio)}
                            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                            role="menuitem"
                          >
                            <Copy className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Duplicate
                          </button>
                          <button
                            onClick={() => handleExport(portfolio, 'html')}
                            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                            role="menuitem"
                          >
                            <Download className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Export as Website
                          </button>
                          <button
                            onClick={() => handleExport(portfolio, 'pdf')}
                            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                            role="menuitem"
                          >
                            <FileText className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Export as PDF
                          </button>
                          {!isArchived && (
                            <button
                              onClick={() => handleArchive(portfolio)}
                              className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                              role="menuitem"
                            >
                              <Archive className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Archive
                            </button>
                          )}
                          <div className="border-t border-border-subtle my-1" />
                          <button
                            onClick={() => { setDeleteConfirmId(portfolio.id); setOpenMenuId(null); }}
                            className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-accent-danger hover:bg-accent-danger/10 transition-colors text-left"
                            role="menuitem"
                          >
                            <Trash2 className="w-4 h-4" aria-hidden="true" /> Delete
                          </button>
                        </div>
                      )}
                    </div>
                  </div>

                  {portfolio.tagline && (
                    <p className="text-sm text-text-tertiary line-clamp-2 mb-3">{portfolio.tagline}</p>
                  )}

                  <div className="mt-auto pt-3 border-t border-gray-50 flex items-center justify-between text-xs text-text-disabled">
                    <span className="flex items-center gap-1">
                      <Clock className="w-3.5 h-3.5" aria-hidden="true" />
                      {formatDate(portfolio.updated_at)}
                    </span>
                    <span className="flex items-center gap-1">
                      <Eye className="w-3.5 h-3.5" aria-hidden="true" />
                      {formatViewCount(portfolio.view_count)}
                    </span>
                  </div>
                </div>
              </article>
            );
          })}
        </div>
      )}

      {/* ══════ Create Modal ══════ */}
      {showCreateModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="create-portfolio-title"
        >
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/40 backdrop-blur-sm"
            onClick={() => setShowCreateModal(false)}
            aria-hidden="true"
          />

          {/* Modal */}
          <div className="relative w-full max-w-lg bg-surface-0 rounded-2xl shadow-2xl overflow-hidden">
            <div className="p-6 pb-0">
              <div className="flex items-center justify-between mb-6">
                <h2 id="create-portfolio-title" className="text-xl font-bold text-text-primary">Create New Portfolio</h2>
                <button
                  onClick={() => setShowCreateModal(false)}
                  className="p-2 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                  aria-label="Close"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <form onSubmit={handleCreate} id="create-portfolio-form">
                {/* Name */}
                <div className="mb-5">
                  <label htmlFor="portfolio-name" className="block text-sm font-semibold text-text-secondary mb-1.5">
                    Portfolio Name
                  </label>
                  <input
                    id="portfolio-name"
                    type="text"
                    value={createName}
                    onChange={(e) => setCreateName(e.target.value)}
                    placeholder="My Amazing Portfolio"
                    className="w-full px-4 py-3 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent transition-shadow"
                    required
                    autoFocus
                  />
                </div>

                {/* Template */}
                <div className="mb-5">
                  <label className="block text-sm font-semibold text-text-secondary mb-2">Template</label>
                  <div className="grid grid-cols-2 gap-2" role="radiogroup" aria-label="Portfolio template">
                    {TEMPLATES.map((tpl) => {
                      const Icon = tpl.icon;
                      return (
                        <button
                          key={tpl.id}
                          type="button"
                          onClick={() => setCreateTemplate(tpl.id)}
                          role="radio"
                          aria-checked={createTemplate === tpl.id}
                          className={`p-3 rounded-xl border-2 text-left transition-all ${
                            createTemplate === tpl.id
                              ? 'border-brand-500 bg-brand-50 ring-2 ring-blue-200'
                              : 'border-border-subtle hover:border-border-default bg-surface-0'
                          }`}
                        >
                          <Icon className={`w-5 h-5 mb-1.5 ${createTemplate === tpl.id ? 'text-brand-600' : 'text-text-disabled'}`} aria-hidden="true" />
                          <div className="text-sm font-semibold text-text-primary">{tpl.name}</div>
                          <div className="text-xs text-text-disabled mt-0.5 line-clamp-2">{tpl.description}</div>
                        </button>
                      );
                    })}
                  </div>
                </div>

                {/* Theme */}
                <div className="mb-2">
                  <label className="block text-sm font-semibold text-text-secondary mb-2">Theme</label>
                  <div className="flex gap-2" role="radiogroup" aria-label="Portfolio theme">
                    {Object.entries(THEME_META).map(([key, meta]) => (
                      <button
                        key={key}
                        type="button"
                        onClick={() => setCreateTheme(key)}
                        role="radio"
                        aria-checked={createTheme === key}
                        aria-label={meta.label}
                        title={meta.label}
                        className={`w-10 h-10 rounded-xl ${meta.color} transition-all ${
                          createTheme === key
                            ? `ring-4 ${meta.ring} scale-110`
                            : 'opacity-60 hover:opacity-100'
                        }`}
                      />
                    ))}
                  </div>
                  <p className="text-xs text-text-disabled mt-1.5">
                    {THEME_META[createTheme]?.label} -- you can change this later
                  </p>
                </div>
              </form>
            </div>

            <div className="flex items-center justify-end gap-3 p-6 pt-4 bg-surface-1/50 border-t border-border-subtle mt-4">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="px-5 py-2.5 text-sm font-medium text-text-secondary hover:text-text-primary rounded-xl hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-gray-300"
              >
                Cancel
              </button>
              <button
                type="submit"
                form="create-portfolio-form"
                disabled={creating || !createName.trim()}
                className="px-6 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl font-medium text-sm hover:from-blue-700 hover:to-indigo-700 transition-all shadow-lg shadow-blue-500/25 disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                {creating ? 'Creating...' : 'Create Portfolio'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ══════ Delete Confirmation Modal ══════ */}
      {deleteConfirmId && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="delete-confirm-title"
          aria-describedby="delete-confirm-desc"
        >
          <div
            className="absolute inset-0 bg-black/40 backdrop-blur-sm"
            onClick={() => setDeleteConfirmId(null)}
            aria-hidden="true"
          />
          <div className="relative w-full max-w-sm bg-surface-0 rounded-2xl shadow-2xl p-6 text-center">
            <div className="inline-flex items-center justify-center w-14 h-14 rounded-full bg-accent-danger/10 mb-4">
              <Trash2 className="w-7 h-7 text-accent-danger" aria-hidden="true" />
            </div>
            <h3 id="delete-confirm-title" className="text-lg font-bold text-text-primary mb-1">Delete Portfolio?</h3>
            <p id="delete-confirm-desc" className="text-sm text-text-tertiary mb-6">
              This action cannot be undone. All sections, artifacts, and comments will be permanently removed.
            </p>
            <div className="flex gap-3">
              <button
                onClick={() => setDeleteConfirmId(null)}
                className="flex-1 px-4 py-2.5 text-sm font-medium text-text-secondary bg-surface-2 rounded-xl hover:bg-border-default transition-colors focus:outline-none focus:ring-2 focus:ring-gray-300"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteConfirmId)}
                className="flex-1 px-4 py-2.5 text-sm font-medium text-white bg-accent-danger rounded-xl hover:bg-accent-danger/90 transition-colors focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2"
              >
                Delete Forever
              </button>
            </div>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default PortfoliosPage;
