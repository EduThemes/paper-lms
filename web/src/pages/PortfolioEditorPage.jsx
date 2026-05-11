import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Save,
  Eye,
  Globe,
  Download,
  FileText,
  Settings,
  Plus,
  Trash2,
  GripVertical,
  ChevronDown,
  ChevronUp,
  Upload,
  Image,
  Link as LinkIcon,
  X,
  ExternalLink,
  Palette,
  Columns,
  LayoutGrid,
  AlignLeft,
  Clock,
  Layers,
  Star,
  Tag,
  MessageSquare,
  EyeOff,
  Check,
  Camera,
  User,
  Mail,
  Linkedin,
  Globe2,
  Code,
  BookOpen,
  Briefcase,
  Award,
  GraduationCap,
  PenTool,
  FolderOpen,
  Lightbulb,
  MoreVertical,
  Copy,
  MoveUp,
  MoveDown,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';

/* ═══════════════════════════════════════════════════════
   THEMES
   ═══════════════════════════════════════════════════════ */
const THEMES = [
  {
    id: 'clean_modern',
    label: 'Clean Modern',
    desc: 'White & blue, sans-serif elegance',
    preview: 'bg-gradient-to-br from-white to-blue-50',
    headerBg: 'bg-surface-0',
    textClass: 'font-sans text-text-primary',
    accentClass: 'text-brand-600',
    cardClass: 'bg-surface-0 border border-border-default shadow-sm',
    heroBg: 'bg-gradient-to-br from-blue-50 via-white to-indigo-50',
  },
  {
    id: 'creative_bold',
    label: 'Creative Bold',
    desc: 'Dark background, vibrant accents',
    preview: 'bg-gradient-to-br from-gray-900 to-purple-900',
    headerBg: 'bg-gray-900',
    textClass: 'font-sans text-gray-100',
    accentClass: 'text-pink-400',
    cardClass: 'bg-gray-800/80 border border-gray-700',
    heroBg: 'bg-gradient-to-br from-gray-900 via-purple-900 to-gray-900',
  },
  {
    id: 'academic_classic',
    label: 'Academic Classic',
    desc: 'Serif fonts, warm cream tones',
    preview: 'bg-gradient-to-br from-amber-50 to-orange-50',
    headerBg: 'bg-accent-warning/10',
    textClass: 'font-serif text-text-primary',
    accentClass: 'text-accent-warning',
    cardClass: 'bg-surface-0 border border-accent-warning/30 shadow-sm',
    heroBg: 'bg-gradient-to-br from-amber-50 via-orange-50 to-yellow-50',
  },
  {
    id: 'minimal_dark',
    label: 'Minimal Dark',
    desc: 'Dark mode, monospace touches',
    preview: 'bg-gradient-to-br from-gray-950 to-gray-800',
    headerBg: 'bg-gray-950',
    textClass: 'font-mono text-gray-200',
    accentClass: 'text-text-disabled',
    cardClass: 'bg-gray-900 border border-gray-800',
    heroBg: 'bg-gradient-to-br from-gray-950 to-gray-900',
  },
  {
    id: 'developer_portfolio',
    label: 'Developer Portfolio',
    desc: 'Terminal-inspired, code aesthetic',
    preview: 'bg-gradient-to-br from-gray-950 to-emerald-950',
    headerBg: 'bg-gray-950',
    textClass: 'font-mono text-emerald-100',
    accentClass: 'text-emerald-400',
    cardClass: 'bg-gray-900/90 border border-emerald-900/50',
    heroBg: 'bg-gradient-to-br from-gray-950 via-emerald-950 to-gray-950',
  },
];

const SECTION_TYPES = [
  { id: 'about',      label: 'About Me',    icon: User },
  { id: 'projects',   label: 'Projects',    icon: FolderOpen },
  { id: 'experience', label: 'Experience',  icon: Briefcase },
  { id: 'education',  label: 'Education',   icon: GraduationCap },
  { id: 'skills',     label: 'Skills',      icon: Award },
  { id: 'gallery',    label: 'Gallery',     icon: Image },
  { id: 'blog',       label: 'Blog',        icon: BookOpen },
  { id: 'custom',     label: 'Custom',      icon: PenTool },
];

const LAYOUT_OPTIONS = [
  { id: 'standard',    label: 'Standard',   icon: AlignLeft },
  { id: 'two-column',  label: 'Two Column', icon: Columns },
  { id: 'grid',        label: 'Grid',       icon: LayoutGrid },
  { id: 'timeline',    label: 'Timeline',   icon: Clock },
  { id: 'masonry',     label: 'Masonry',    icon: Layers },
];

/* ═══════════════════════════════════════════════════════
   Component: PortfolioEditorPage
   ═══════════════════════════════════════════════════════ */
const PortfolioEditorPage = () => {
  const { portfolioId } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const autoSaveTimerRef = useRef(null);

  /* ── Core state ── */
  const [portfolio, setPortfolio] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(false);
  const [lastSaved, setLastSaved] = useState(null);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);

  /* ── Panel state ── */
  const [activePanel, setActivePanel] = useState(null); // 'theme' | 'settings' | 'artifacts' | null
  const [editingSectionId, setEditingSectionId] = useState(null);
  const [showAddSection, setShowAddSection] = useState(false);
  const [showPreview, setShowPreview] = useState(true);

  /* ── Artifact modal state ── */
  const [artifactModal, setArtifactModal] = useState(false);
  const [artifactData, setArtifactData] = useState({
    title: '',
    description: '',
    tags: '',
    type: 'file', // file | link | course_submission
    url: '',
    file: null,
    course_submission_id: '',
    featured: false,
    reflection: '',
    section_id: null,
  });

  /* ── Settings state ── */
  const [settings, setSettings] = useState({
    contact_email: '',
    linkedin_url: '',
    website_url: '',
    custom_css: '',
  });

  /* ────────────────────────────────────
     Fetch portfolio
     ──────────────────────────────────── */
  const fetchPortfolio = useCallback(async () => {
    try {
      const data = await api.getPortfolio(portfolioId);
      setPortfolio(data);
      setSettings({
        contact_email: data.contact_email || '',
        linkedin_url: data.linkedin_url || '',
        website_url: data.website_url || '',
        custom_css: data.custom_css || '',
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [portfolioId]);

  useEffect(() => {
    fetchPortfolio();
  }, [fetchPortfolio]);

  /* ────────────────────────────────────
     Auto-save (debounced)
     ──────────────────────────────────── */
  const savePortfolio = useCallback(async (data) => {
    setSaving(true);
    try {
      const updated = await api.updatePortfolio(portfolioId, data);
      setPortfolio(updated);
      setLastSaved(new Date());
      setHasUnsavedChanges(false);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }, [portfolioId]);

  const scheduleAutoSave = useCallback((updatedFields) => {
    setHasUnsavedChanges(true);
    if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);
    autoSaveTimerRef.current = setTimeout(() => {
      savePortfolio(updatedFields);
    }, 1500);
  }, [savePortfolio]);

  useEffect(() => {
    return () => {
      if (autoSaveTimerRef.current) clearTimeout(autoSaveTimerRef.current);
    };
  }, []);

  /* ────────────────────────────────────
     Field update helpers
     ──────────────────────────────────── */
  const updateField = (field, value) => {
    setPortfolio((prev) => ({ ...prev, [field]: value }));
    scheduleAutoSave({ [field]: value });
  };

  const updateTheme = (themeId) => {
    setPortfolio((prev) => ({ ...prev, theme: themeId }));
    savePortfolio({ theme: themeId });
  };

  /* ────────────────────────────────────
     Section operations
     ──────────────────────────────────── */
  const addSection = async (type) => {
    setShowAddSection(false);
    try {
      const sectionType = SECTION_TYPES.find((s) => s.id === type);
      const data = await api.addPortfolioSection(portfolioId, {
        type,
        title: sectionType?.label || 'New Section',
        layout: 'standard',
        visible: true,
        content: '',
        order: (portfolio.sections?.length || 0) + 1,
      });
      setPortfolio((prev) => ({
        ...prev,
        sections: [...(prev.sections || []), data],
      }));
      setEditingSectionId(data.id);
    } catch (err) {
      setError(err.message);
    }
  };

  const updateSection = async (sectionId, updates) => {
    try {
      const data = await api.updatePortfolioSection(portfolioId, sectionId, updates);
      setPortfolio((prev) => ({
        ...prev,
        sections: prev.sections.map((s) => (s.id === sectionId ? data : s)),
      }));
    } catch (err) {
      setError(err.message);
    }
  };

  const deleteSection = async (sectionId) => {
    if (!window.confirm('Remove this section? Any artifacts in it will be unlinked.')) return;
    try {
      await api.deletePortfolioSection(portfolioId, sectionId);
      setPortfolio((prev) => ({
        ...prev,
        sections: prev.sections.filter((s) => s.id !== sectionId),
      }));
      if (editingSectionId === sectionId) setEditingSectionId(null);
    } catch (err) {
      setError(err.message);
    }
  };

  const moveSection = async (sectionId, direction) => {
    const sections = [...(portfolio.sections || [])];
    const idx = sections.findIndex((s) => s.id === sectionId);
    if (idx < 0) return;
    const newIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (newIdx < 0 || newIdx >= sections.length) return;
    [sections[idx], sections[newIdx]] = [sections[newIdx], sections[idx]];
    sections.forEach((s, i) => { s.order = i + 1; });
    setPortfolio((prev) => ({ ...prev, sections }));
    try {
      await api.updatePortfolioSection(portfolioId, sections[idx].id, { order: sections[idx].order });
      await api.updatePortfolioSection(portfolioId, sections[newIdx].id, { order: sections[newIdx].order });
    } catch (err) {
      setError(err.message);
    }
  };

  /* ────────────────────────────────────
     Artifacts
     ──────────────────────────────────── */
  const handleAddArtifact = async (e) => {
    e.preventDefault();
    try {
      const payload = {
        title: artifactData.title,
        description: artifactData.description,
        tags: artifactData.tags.split(',').map((t) => t.trim()).filter(Boolean),
        type: artifactData.type,
        url: artifactData.url,
        featured: artifactData.featured,
        reflection: artifactData.reflection,
        section_id: artifactData.section_id,
        course_submission_id: artifactData.course_submission_id || undefined,
      };

      if (artifactData.type === 'file' && artifactData.file) {
        const formData = new FormData();
        formData.append('file', artifactData.file);
        Object.entries(payload).forEach(([k, v]) => {
          if (v !== undefined && v !== null) {
            formData.append(k, typeof v === 'object' ? JSON.stringify(v) : v);
          }
        });
        await api.uploadPortfolioArtifact(portfolioId, formData);
      } else {
        await api.addPortfolioArtifact(portfolioId, payload);
      }

      setArtifactModal(false);
      setArtifactData({
        title: '', description: '', tags: '', type: 'file', url: '', file: null,
        course_submission_id: '', featured: false, reflection: '', section_id: null,
      });
      await fetchPortfolio();
    } catch (err) {
      setError(err.message);
    }
  };

  /* ────────────────────────────────────
     Publish / Export
     ──────────────────────────────────── */
  const handlePublish = async () => {
    try {
      await api.publishPortfolio(portfolioId);
      await fetchPortfolio();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUnpublish = async () => {
    try {
      await api.unpublishPortfolio(portfolioId);
      await fetchPortfolio();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleExportHTML = async () => {
    try {
      const response = await api.exportPortfolioHTML(portfolioId);
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${portfolio.title || 'portfolio'}.zip`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleExportPDF = async () => {
    try {
      const response = await api.exportPortfolioPDF(portfolioId);
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${portfolio.title || 'portfolio'}.pdf`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSaveSettings = async () => {
    await savePortfolio(settings);
    setActivePanel(null);
  };

  /* ────────────────────────────────────
     Current theme helper
     ──────────────────────────────────── */
  const currentTheme = THEMES.find((t) => t.id === portfolio?.theme) || THEMES[0];

  /* ════════════════════════════════════
     Render helpers
     ════════════════════════════════════ */

  /* Loading */
  if (loading) {
    return (
      <Layout>
        <div className="flex flex-col items-center justify-center py-24" role="status" aria-label="Loading editor">
          <div className="relative w-16 h-16 mb-6">
            <div className="absolute inset-0 rounded-full border-4 border-blue-100" />
            <div className="absolute inset-0 rounded-full border-4 border-brand-500 border-t-transparent animate-spin" />
          </div>
          <p className="text-text-tertiary text-lg font-medium">Loading portfolio editor...</p>
        </div>
      </Layout>
    );
  }

  /* Error (no portfolio loaded) */
  if (error && !portfolio) {
    return (
      <Layout>
        <div className="text-center py-16">
          <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-accent-danger/10 mb-4">
            <X className="w-8 h-8 text-accent-danger" />
          </div>
          <h2 className="text-xl font-semibold text-text-primary mb-2">Could not load portfolio</h2>
          <p className="text-text-tertiary mb-6">{error}</p>
          <Link to="/portfolios" className="text-brand-600 hover:underline font-medium">Back to Portfolios</Link>
        </div>
      </Layout>
    );
  }

  if (!portfolio) return null;

  const isPublished = portfolio.status === 'published';
  const sections = portfolio.sections || [];

  return (
    <div className="min-h-screen bg-surface-1 flex flex-col">
      {/* ═══════ Top Toolbar ═══════ */}
      <header className="bg-surface-0 border-b border-border-default sticky top-0 z-40" role="banner">
        <div className="flex items-center justify-between px-4 h-14">
          {/* Left */}
          <div className="flex items-center gap-3">
            <Link
              to="/portfolios"
              className="p-2 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
              aria-label="Back to portfolios"
            >
              <ArrowLeft className="w-5 h-5" />
            </Link>
            <div className="h-6 w-px bg-border-default" aria-hidden="true" />
            <span className="text-sm font-medium text-text-tertiary hidden sm:inline">Portfolio Editor</span>
          </div>

          {/* Center -- Save status */}
          <div className="flex items-center gap-2 text-xs text-text-disabled" aria-live="polite">
            {saving ? (
              <><div className="w-3 h-3 border-2 border-blue-400 border-t-transparent rounded-full animate-spin" aria-hidden="true" /> Saving...</>
            ) : hasUnsavedChanges ? (
              <><div className="w-2 h-2 rounded-full bg-amber-400" aria-hidden="true" /> Unsaved changes</>
            ) : lastSaved ? (
              <><Check className="w-3.5 h-3.5 text-accent-success" aria-hidden="true" /> Saved</>
            ) : null}
          </div>

          {/* Right -- Actions */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowPreview(!showPreview)}
              className={`hidden lg:flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                showPreview ? 'bg-brand-50 text-brand-700' : 'text-text-tertiary hover:bg-surface-2'
              } focus:outline-none focus:ring-2 focus:ring-brand-500`}
              aria-pressed={showPreview}
              aria-label="Toggle live preview"
            >
              <Eye className="w-4 h-4" aria-hidden="true" />
              Preview
            </button>
            <button
              onClick={() => setActivePanel(activePanel === 'settings' ? null : 'settings')}
              className={`p-2 rounded-lg transition-colors ${
                activePanel === 'settings' ? 'bg-surface-2 text-text-primary' : 'text-text-disabled hover:text-text-secondary hover:bg-surface-2'
              } focus:outline-none focus:ring-2 focus:ring-brand-500`}
              aria-label="Portfolio settings"
              aria-pressed={activePanel === 'settings'}
            >
              <Settings className="w-5 h-5" />
            </button>
            <div className="h-6 w-px bg-border-default hidden sm:block" aria-hidden="true" />

            {/* Export dropdown */}
            <div className="relative group">
              <button
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-text-tertiary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                aria-haspopup="true"
                aria-label="Export portfolio"
              >
                <Download className="w-4 h-4" aria-hidden="true" />
                <span className="hidden sm:inline">Export</span>
                <ChevronDown className="w-3 h-3" aria-hidden="true" />
              </button>
              <div className="absolute right-0 top-full mt-1 w-48 bg-surface-0 rounded-xl shadow-xl border border-border-subtle py-1.5 hidden group-focus-within:block group-hover:block z-30" role="menu">
                <button
                  onClick={handleExportHTML}
                  className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                  role="menuitem"
                >
                  <Code className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Export as Website (ZIP)
                </button>
                <button
                  onClick={handleExportPDF}
                  className="flex items-center gap-2.5 w-full px-4 py-2 text-sm text-text-secondary hover:bg-surface-1 transition-colors text-left"
                  role="menuitem"
                >
                  <FileText className="w-4 h-4 text-text-disabled" aria-hidden="true" /> Export as PDF
                </button>
              </div>
            </div>

            {/* Preview in new tab */}
            {isPublished && (
              <a
                href={`/p/${portfolio.slug || portfolio.id}`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-text-tertiary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                aria-label="Open public portfolio in new tab"
              >
                <ExternalLink className="w-4 h-4" aria-hidden="true" />
                <span className="hidden sm:inline">View Live</span>
              </a>
            )}

            {/* Publish button */}
            {isPublished ? (
              <button
                onClick={handleUnpublish}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-semibold bg-accent-success/10 text-accent-success border border-accent-success/30 hover:bg-accent-success/20 transition-colors focus:outline-none focus:ring-2 focus:ring-green-500"
              >
                <Globe className="w-4 h-4" aria-hidden="true" />
                Published
              </button>
            ) : (
              <button
                onClick={handlePublish}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-sm font-semibold bg-gradient-to-r from-blue-600 to-indigo-600 text-white hover:from-blue-700 hover:to-indigo-700 transition-all shadow-md shadow-blue-500/25 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                <Globe className="w-4 h-4" aria-hidden="true" />
                Publish
              </button>
            )}
          </div>
        </div>
      </header>

      {/* ── Error banner ── */}
      {error && (
        <div className="bg-accent-danger/10 border-b border-accent-danger/30 px-4 py-2.5 flex items-center gap-3" role="alert">
          <X className="w-4 h-4 text-accent-danger shrink-0" aria-hidden="true" />
          <p className="text-sm text-accent-danger flex-1">{error}</p>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-accent-danger" aria-label="Dismiss error">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* ═══════ Main Editor Area ═══════ */}
      <div className="flex-1 flex overflow-hidden">

        {/* ──────── LEFT: Editor Panel ──────── */}
        <div className={`flex-1 overflow-y-auto ${showPreview ? 'lg:w-1/2' : 'w-full'}`}>
          <div className="max-w-2xl mx-auto p-6 space-y-6">

            {/* ── Cover Image Area ── */}
            <section aria-labelledby="cover-section-label" className="relative rounded-2xl overflow-hidden group">
              <h2 id="cover-section-label" className="sr-only">Cover Image</h2>
              <div className={`h-48 ${currentTheme.heroBg} flex items-center justify-center relative`}>
                {portfolio.cover_image_url ? (
                  <img src={portfolio.cover_image_url} alt="Portfolio cover" className="w-full h-full object-cover" />
                ) : (
                  <div className="text-center">
                    <Camera className="w-10 h-10 text-text-disabled/50 mx-auto mb-2" aria-hidden="true" />
                    <p className="text-sm text-text-disabled">Add a cover image</p>
                  </div>
                )}
                <button
                  onClick={() => {/* trigger file picker */}}
                  className="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center opacity-0 group-hover:opacity-100"
                  aria-label="Change cover image"
                >
                  <span className="px-4 py-2 bg-surface-0 rounded-lg text-sm font-medium shadow-lg">Change Cover</span>
                </button>
              </div>

              {/* Avatar */}
              <div className="absolute -bottom-8 left-6">
                <div className="w-20 h-20 rounded-2xl border-4 border-white bg-surface-2 shadow-lg flex items-center justify-center overflow-hidden group/avatar">
                  {portfolio.avatar_url ? (
                    <img src={portfolio.avatar_url} alt="Avatar" className="w-full h-full object-cover" />
                  ) : (
                    <User className="w-8 h-8 text-gray-300" aria-hidden="true" />
                  )}
                  <button
                    className="absolute inset-0 bg-black/0 group-hover/avatar:bg-black/30 transition-colors flex items-center justify-center opacity-0 group-hover/avatar:opacity-100 rounded-2xl"
                    aria-label="Change avatar"
                  >
                    <Camera className="w-5 h-5 text-white" />
                  </button>
                </div>
              </div>
            </section>

            {/* ── Title & Tagline ── */}
            <section className="pt-6" aria-labelledby="title-section-label">
              <h2 id="title-section-label" className="sr-only">Portfolio Title and Tagline</h2>
              <input
                type="text"
                value={portfolio.title || ''}
                onChange={(e) => updateField('title', e.target.value)}
                placeholder="Portfolio Title"
                className="w-full text-3xl font-bold text-text-primary bg-transparent border-none outline-none placeholder-gray-300 focus:ring-0"
                aria-label="Portfolio title"
              />
              <input
                type="text"
                value={portfolio.tagline || ''}
                onChange={(e) => updateField('tagline', e.target.value)}
                placeholder="Your tagline -- describe who you are in one line"
                className="w-full mt-2 text-lg text-text-tertiary bg-transparent border-none outline-none placeholder-gray-300 focus:ring-0"
                aria-label="Portfolio tagline"
              />
            </section>

            {/* ── Theme Selector ── */}
            <section aria-labelledby="theme-selector-label" className="bg-surface-0 rounded-2xl border border-border-subtle p-5">
              <h2 id="theme-selector-label" className="text-sm font-semibold text-text-secondary mb-3 flex items-center gap-2">
                <Palette className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                Theme
              </h2>
              <div className="grid grid-cols-5 gap-2" role="radiogroup" aria-label="Select theme">
                {THEMES.map((theme) => (
                  <button
                    key={theme.id}
                    onClick={() => updateTheme(theme.id)}
                    role="radio"
                    aria-checked={portfolio.theme === theme.id}
                    aria-label={theme.label}
                    className={`relative rounded-xl overflow-hidden aspect-[4/3] border-2 transition-all ${
                      portfolio.theme === theme.id
                        ? 'border-brand-500 ring-2 ring-blue-200 scale-105'
                        : 'border-border-default hover:border-border-strong opacity-70 hover:opacity-100'
                    }`}
                  >
                    <div className={`absolute inset-0 ${theme.preview}`} />
                    <div className="absolute inset-x-0 bottom-0 bg-surface-0/90 backdrop-blur-sm px-1.5 py-1">
                      <span className="text-[10px] font-medium text-text-secondary leading-tight block truncate">{theme.label}</span>
                    </div>
                    {portfolio.theme === theme.id && (
                      <div className="absolute top-1 right-1 w-4 h-4 bg-brand-500 rounded-full flex items-center justify-center">
                        <Check className="w-2.5 h-2.5 text-white" aria-hidden="true" />
                      </div>
                    )}
                  </button>
                ))}
              </div>
            </section>

            {/* ══════ Sections Manager ══════ */}
            <section aria-labelledby="sections-label">
              <h2 id="sections-label" className="text-sm font-semibold text-text-secondary mb-3 flex items-center gap-2">
                <Layers className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                Sections
                <span className="text-xs text-text-disabled font-normal ml-1">({sections.length})</span>
              </h2>

              <div className="space-y-3" role="list" aria-label="Portfolio sections">
                {sections.sort((a, b) => (a.order || 0) - (b.order || 0)).map((section, idx) => {
                  const SectionIcon = SECTION_TYPES.find((s) => s.id === section.type)?.icon || PenTool;
                  const isEditing = editingSectionId === section.id;

                  return (
                    <div
                      key={section.id}
                      role="listitem"
                      className={`bg-surface-0 rounded-xl border transition-all ${
                        isEditing ? 'border-blue-300 shadow-lg ring-2 ring-blue-100' : 'border-border-subtle shadow-sm hover:shadow-md'
                      }`}
                    >
                      {/* Section header */}
                      <div className="flex items-center gap-2 p-3">
                        <GripVertical className="w-4 h-4 text-gray-300 shrink-0 cursor-grab" aria-hidden="true" />
                        <SectionIcon className="w-4 h-4 text-text-disabled shrink-0" aria-hidden="true" />
                        <button
                          onClick={() => setEditingSectionId(isEditing ? null : section.id)}
                          className="flex-1 text-left text-sm font-medium text-text-primary hover:text-brand-600 transition-colors truncate focus:outline-none focus:underline"
                          aria-expanded={isEditing}
                          aria-controls={`section-editor-${section.id}`}
                        >
                          {section.title || 'Untitled Section'}
                        </button>

                        <div className="flex items-center gap-1 shrink-0">
                          {/* Visibility toggle */}
                          <button
                            onClick={() => updateSection(section.id, { visible: !section.visible })}
                            className={`p-1.5 rounded-lg transition-colors ${
                              section.visible ? 'text-text-disabled hover:text-text-secondary' : 'text-red-400 hover:text-accent-danger'
                            } focus:outline-none focus:ring-2 focus:ring-brand-500`}
                            aria-label={section.visible ? 'Hide section' : 'Show section'}
                            title={section.visible ? 'Visible' : 'Hidden'}
                          >
                            {section.visible ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
                          </button>
                          {/* Move up */}
                          <button
                            onClick={() => moveSection(section.id, 'up')}
                            disabled={idx === 0}
                            className="p-1.5 rounded-lg text-text-disabled hover:text-text-secondary disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                            aria-label="Move section up"
                          >
                            <MoveUp className="w-4 h-4" />
                          </button>
                          {/* Move down */}
                          <button
                            onClick={() => moveSection(section.id, 'down')}
                            disabled={idx === sections.length - 1}
                            className="p-1.5 rounded-lg text-text-disabled hover:text-text-secondary disabled:opacity-30 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                            aria-label="Move section down"
                          >
                            <MoveDown className="w-4 h-4" />
                          </button>
                          {/* Delete */}
                          <button
                            onClick={() => deleteSection(section.id)}
                            className="p-1.5 rounded-lg text-text-disabled hover:text-accent-danger transition-colors focus:outline-none focus:ring-2 focus:ring-red-500"
                            aria-label={`Delete ${section.title} section`}
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>

                      {/* Expanded section editor */}
                      {isEditing && (
                        <div id={`section-editor-${section.id}`} className="border-t border-border-subtle p-4 space-y-4">
                          {/* Title */}
                          <div>
                            <label htmlFor={`sec-title-${section.id}`} className="block text-xs font-semibold text-text-tertiary uppercase tracking-wide mb-1">
                              Section Title
                            </label>
                            <input
                              id={`sec-title-${section.id}`}
                              type="text"
                              value={section.title || ''}
                              onChange={(e) => updateSection(section.id, { title: e.target.value })}
                              className="w-full px-3 py-2 border border-border-default rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                            />
                          </div>

                          {/* Type */}
                          <div>
                            <label htmlFor={`sec-type-${section.id}`} className="block text-xs font-semibold text-text-tertiary uppercase tracking-wide mb-1">
                              Section Type
                            </label>
                            <select
                              id={`sec-type-${section.id}`}
                              value={section.type}
                              onChange={(e) => updateSection(section.id, { type: e.target.value })}
                              className="w-full px-3 py-2 border border-border-default rounded-lg text-sm bg-surface-0 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                            >
                              {SECTION_TYPES.map((t) => (
                                <option key={t.id} value={t.id}>{t.label}</option>
                              ))}
                            </select>
                          </div>

                          {/* Layout */}
                          <div>
                            <label className="block text-xs font-semibold text-text-tertiary uppercase tracking-wide mb-2">
                              Layout
                            </label>
                            <div className="flex gap-2" role="radiogroup" aria-label="Section layout">
                              {LAYOUT_OPTIONS.map((lo) => {
                                const LoIcon = lo.icon;
                                return (
                                  <button
                                    key={lo.id}
                                    onClick={() => updateSection(section.id, { layout: lo.id })}
                                    role="radio"
                                    aria-checked={section.layout === lo.id}
                                    className={`flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium border transition-all ${
                                      section.layout === lo.id
                                        ? 'border-brand-500 bg-brand-50 text-brand-700'
                                        : 'border-border-default text-text-tertiary hover:border-border-strong'
                                    }`}
                                  >
                                    <LoIcon className="w-3.5 h-3.5" aria-hidden="true" />
                                    {lo.label}
                                  </button>
                                );
                              })}
                            </div>
                          </div>

                          {/* Content */}
                          <div>
                            <label htmlFor={`sec-content-${section.id}`} className="block text-xs font-semibold text-text-tertiary uppercase tracking-wide mb-1">
                              Content
                            </label>
                            <textarea
                              id={`sec-content-${section.id}`}
                              value={section.content || ''}
                              onChange={(e) => updateSection(section.id, { content: e.target.value })}
                              rows={6}
                              placeholder="Write your content here... Markdown is supported."
                              className="w-full px-3 py-2 border border-border-default rounded-lg text-sm resize-y focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                            />
                          </div>

                          {/* Add artifact to this section */}
                          <button
                            onClick={() => {
                              setArtifactData((prev) => ({ ...prev, section_id: section.id }));
                              setArtifactModal(true);
                            }}
                            className="inline-flex items-center gap-1.5 px-3 py-2 text-sm font-medium text-brand-600 bg-brand-50 rounded-lg hover:bg-brand-100 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                          >
                            <Plus className="w-4 h-4" aria-hidden="true" />
                            Add Artifact to Section
                          </button>

                          {/* Section artifacts preview */}
                          {(portfolio.artifacts || []).filter((a) => a.section_id === section.id).length > 0 && (
                            <div className="mt-2">
                              <p className="text-xs font-semibold text-text-tertiary uppercase tracking-wide mb-2">Artifacts in this section</p>
                              <div className="space-y-1.5">
                                {(portfolio.artifacts || []).filter((a) => a.section_id === section.id).map((artifact) => (
                                  <div key={artifact.id} className="flex items-center gap-2 p-2 bg-surface-1 rounded-lg text-sm">
                                    {artifact.featured && <Star className="w-3.5 h-3.5 text-amber-500 shrink-0" aria-label="Featured" />}
                                    <span className="flex-1 truncate text-text-secondary">{artifact.title}</span>
                                    {artifact.tags?.length > 0 && (
                                      <div className="flex gap-1 shrink-0">
                                        {artifact.tags.slice(0, 2).map((tag) => (
                                          <span key={tag} className="px-1.5 py-0.5 bg-border-default text-text-tertiary rounded text-[10px]">{tag}</span>
                                        ))}
                                      </div>
                                    )}
                                  </div>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>

              {/* Add Section button */}
              <div className="mt-4 relative">
                <button
                  onClick={() => setShowAddSection(!showAddSection)}
                  className="w-full flex items-center justify-center gap-2 p-4 border-2 border-dashed border-border-default rounded-xl text-text-disabled hover:text-brand-600 hover:border-blue-300 hover:bg-brand-50/50 transition-all font-medium text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                  aria-expanded={showAddSection}
                  aria-haspopup="true"
                >
                  <Plus className="w-5 h-5" aria-hidden="true" />
                  Add Section
                </button>

                {showAddSection && (
                  <div className="absolute left-0 right-0 top-full mt-2 bg-surface-0 rounded-xl shadow-xl border border-border-subtle p-3 z-20 grid grid-cols-2 gap-2" role="menu">
                    {SECTION_TYPES.map((st) => {
                      const StIcon = st.icon;
                      return (
                        <button
                          key={st.id}
                          onClick={() => addSection(st.id)}
                          className="flex items-center gap-2.5 p-3 rounded-xl hover:bg-brand-50 transition-colors text-sm text-text-secondary font-medium text-left focus:outline-none focus:ring-2 focus:ring-brand-500"
                          role="menuitem"
                        >
                          <StIcon className="w-5 h-5 text-text-disabled" aria-hidden="true" />
                          {st.label}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </section>

            {/* ── General Artifacts ── */}
            <section aria-labelledby="artifacts-label" className="bg-surface-0 rounded-2xl border border-border-subtle p-5">
              <div className="flex items-center justify-between mb-3">
                <h2 id="artifacts-label" className="text-sm font-semibold text-text-secondary flex items-center gap-2">
                  <FolderOpen className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                  All Artifacts
                  <span className="text-xs text-text-disabled font-normal">({(portfolio.artifacts || []).length})</span>
                </h2>
                <button
                  onClick={() => { setArtifactData((p) => ({ ...p, section_id: null })); setArtifactModal(true); }}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-brand-600 hover:bg-brand-50 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <Plus className="w-4 h-4" aria-hidden="true" />
                  Add
                </button>
              </div>

              {(portfolio.artifacts || []).length === 0 ? (
                <div className="text-center py-8">
                  <Upload className="w-8 h-8 text-gray-300 mx-auto mb-2" aria-hidden="true" />
                  <p className="text-sm text-text-disabled">No artifacts yet. Upload files, link external work, or import from your courses.</p>
                </div>
              ) : (
                <div className="space-y-2 max-h-64 overflow-y-auto">
                  {(portfolio.artifacts || []).map((artifact) => (
                    <div key={artifact.id} className="flex items-center gap-3 p-3 bg-surface-1 rounded-xl hover:bg-surface-2 transition-colors">
                      <div className="w-10 h-10 rounded-lg bg-surface-0 border border-border-default flex items-center justify-center shrink-0">
                        {artifact.type === 'link' ? (
                          <LinkIcon className="w-4 h-4 text-brand-500" aria-hidden="true" />
                        ) : artifact.type === 'course_submission' ? (
                          <BookOpen className="w-4 h-4 text-purple-500" aria-hidden="true" />
                        ) : (
                          <FileText className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-text-primary truncate">{artifact.title}</p>
                        <p className="text-xs text-text-disabled truncate">{artifact.description || artifact.url || 'No description'}</p>
                      </div>
                      {artifact.featured && <Star className="w-4 h-4 text-amber-500 shrink-0" aria-label="Featured artifact" />}
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        </div>

        {/* ──────── RIGHT: Live Preview ──────── */}
        {showPreview && (
          <div className="hidden lg:block lg:w-1/2 border-l border-border-default overflow-y-auto bg-surface-2" aria-label="Live portfolio preview">
            <div className="sticky top-0 bg-surface-2 border-b border-border-default px-4 py-2 flex items-center justify-between z-10">
              <span className="text-xs font-semibold text-text-tertiary uppercase tracking-wide">Live Preview</span>
              <span className="text-[10px] text-text-disabled">{currentTheme.label}</span>
            </div>
            <div className={`min-h-full ${currentTheme.heroBg}`}>
              {/* Preview: Hero */}
              <div className="relative px-8 pt-16 pb-12 text-center">
                <div className="w-20 h-20 rounded-2xl bg-surface-0/80 shadow-lg mx-auto mb-4 flex items-center justify-center overflow-hidden">
                  {portfolio.avatar_url ? (
                    <img src={portfolio.avatar_url} alt="" className="w-full h-full object-cover" />
                  ) : (
                    <User className="w-10 h-10 text-gray-300" aria-hidden="true" />
                  )}
                </div>
                <h2 className={`text-2xl font-bold ${
                  currentTheme.id === 'creative_bold' || currentTheme.id === 'minimal_dark' || currentTheme.id === 'developer_portfolio'
                    ? 'text-white'
                    : 'text-text-primary'
                }`}>
                  {portfolio.title || 'Your Portfolio Title'}
                </h2>
                <p className={`mt-2 text-sm ${
                  currentTheme.id === 'creative_bold' || currentTheme.id === 'minimal_dark' || currentTheme.id === 'developer_portfolio'
                    ? 'text-gray-300'
                    : 'text-text-tertiary'
                }`}>
                  {portfolio.tagline || 'Your tagline goes here'}
                </p>
              </div>

              {/* Preview: Sections */}
              <div className="px-6 pb-12 space-y-6">
                {sections.filter((s) => s.visible !== false).sort((a, b) => (a.order || 0) - (b.order || 0)).map((section) => {
                  const isDark = currentTheme.id === 'creative_bold' || currentTheme.id === 'minimal_dark' || currentTheme.id === 'developer_portfolio';
                  return (
                    <div key={section.id} className={`${currentTheme.cardClass} rounded-xl p-6`}>
                      <h3 className={`text-lg font-bold mb-3 ${isDark ? 'text-white' : 'text-text-primary'}`}>
                        {section.title}
                      </h3>
                      {section.content ? (
                        <p className={`text-sm leading-relaxed ${isDark ? 'text-gray-300' : 'text-text-secondary'}`}>
                          {section.content}
                        </p>
                      ) : (
                        <p className={`text-sm italic ${isDark ? 'text-text-tertiary' : 'text-text-disabled'}`}>
                          No content yet. Click to edit this section.
                        </p>
                      )}

                      {/* Preview: Section artifacts */}
                      {(portfolio.artifacts || []).filter((a) => a.section_id === section.id).length > 0 && (
                        <div className={`mt-4 pt-4 border-t ${isDark ? 'border-gray-700' : 'border-border-subtle'}`}>
                          <div className={`grid ${section.layout === 'grid' || section.layout === 'masonry' ? 'grid-cols-2' : 'grid-cols-1'} gap-3`}>
                            {(portfolio.artifacts || []).filter((a) => a.section_id === section.id).map((artifact) => (
                              <div key={artifact.id} className={`p-3 rounded-lg ${isDark ? 'bg-gray-800/50 hover:bg-gray-800' : 'bg-surface-1 hover:bg-surface-2'} transition-colors`}>
                                <p className={`text-sm font-medium ${isDark ? 'text-gray-200' : 'text-text-primary'}`}>{artifact.title}</p>
                                {artifact.description && (
                                  <p className={`text-xs mt-1 ${isDark ? 'text-text-disabled' : 'text-text-tertiary'}`}>{artifact.description}</p>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      <div className={`mt-3 text-xs ${isDark ? 'text-text-tertiary' : 'text-text-disabled'}`}>
                        {LAYOUT_OPTIONS.find((l) => l.id === section.layout)?.label || 'Standard'} layout
                      </div>
                    </div>
                  );
                })}

                {sections.filter((s) => s.visible !== false).length === 0 && (
                  <div className={`text-center py-12 ${
                    currentTheme.id === 'creative_bold' || currentTheme.id === 'minimal_dark' || currentTheme.id === 'developer_portfolio'
                      ? 'text-text-tertiary'
                      : 'text-text-disabled'
                  }`}>
                    <Layers className="w-10 h-10 mx-auto mb-3 opacity-50" aria-hidden="true" />
                    <p className="text-sm">Add sections to build your portfolio</p>
                  </div>
                )}
              </div>

              {/* Preview: Footer */}
              <div className={`border-t px-6 py-4 text-center text-xs ${
                currentTheme.id === 'creative_bold' || currentTheme.id === 'minimal_dark' || currentTheme.id === 'developer_portfolio'
                  ? 'border-gray-800 text-text-secondary'
                  : 'border-border-default text-text-disabled'
              }`}>
                Powered by Paper LMS
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ══════ Settings Slide-over ══════ */}
      {activePanel === 'settings' && (
        <div className="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-labelledby="settings-title">
          <div className="absolute inset-0 bg-black/30" onClick={() => setActivePanel(null)} aria-hidden="true" />
          <div className="absolute right-0 top-0 bottom-0 w-full max-w-md bg-surface-0 shadow-2xl flex flex-col">
            <div className="flex items-center justify-between p-5 border-b border-border-subtle">
              <h2 id="settings-title" className="text-lg font-bold text-text-primary">Portfolio Settings</h2>
              <button
                onClick={() => setActivePanel(null)}
                className="p-2 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                aria-label="Close settings"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-5 space-y-5">
              {/* Contact email */}
              <div>
                <label htmlFor="settings-email" className="flex items-center gap-2 text-sm font-semibold text-text-secondary mb-1.5">
                  <Mail className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                  Contact Email
                </label>
                <input
                  id="settings-email"
                  type="email"
                  value={settings.contact_email}
                  onChange={(e) => setSettings((s) => ({ ...s, contact_email: e.target.value }))}
                  placeholder="your@email.com"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                />
              </div>

              {/* LinkedIn */}
              <div>
                <label htmlFor="settings-linkedin" className="flex items-center gap-2 text-sm font-semibold text-text-secondary mb-1.5">
                  <Linkedin className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                  LinkedIn URL
                </label>
                <input
                  id="settings-linkedin"
                  type="url"
                  value={settings.linkedin_url}
                  onChange={(e) => setSettings((s) => ({ ...s, linkedin_url: e.target.value }))}
                  placeholder="https://linkedin.com/in/yourprofile"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                />
              </div>

              {/* Website */}
              <div>
                <label htmlFor="settings-website" className="flex items-center gap-2 text-sm font-semibold text-text-secondary mb-1.5">
                  <Globe2 className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                  Personal Website
                </label>
                <input
                  id="settings-website"
                  type="url"
                  value={settings.website_url}
                  onChange={(e) => setSettings((s) => ({ ...s, website_url: e.target.value }))}
                  placeholder="https://yourwebsite.com"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                />
              </div>

              {/* Custom CSS */}
              <div>
                <label htmlFor="settings-css" className="flex items-center gap-2 text-sm font-semibold text-text-secondary mb-1.5">
                  <Code className="w-4 h-4 text-text-disabled" aria-hidden="true" />
                  Custom CSS
                </label>
                <textarea
                  id="settings-css"
                  value={settings.custom_css}
                  onChange={(e) => setSettings((s) => ({ ...s, custom_css: e.target.value }))}
                  rows={8}
                  placeholder="/* Add custom styles to your public portfolio */&#10;.portfolio-hero { }&#10;.portfolio-section { }"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent resize-y"
                />
                <p className="text-xs text-text-disabled mt-1">These styles will be applied to your public portfolio page only.</p>
              </div>

              {/* Public URL info */}
              {portfolio.status === 'published' && (
                <div className="bg-accent-success/10 border border-accent-success/30 rounded-xl p-4">
                  <p className="text-sm font-semibold text-accent-success mb-1">Public URL</p>
                  <div className="flex items-center gap-2">
                    <code className="text-xs text-accent-success bg-accent-success/20 px-2 py-1 rounded flex-1 truncate">
                      {window.location.origin}/p/{portfolio.slug || portfolio.id}
                    </code>
                    <button
                      onClick={() => navigator.clipboard.writeText(`${window.location.origin}/p/${portfolio.slug || portfolio.id}`)}
                      className="p-1.5 text-accent-success hover:bg-accent-success/20 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-green-500"
                      aria-label="Copy public URL"
                    >
                      <Copy className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              )}
            </div>

            <div className="p-5 border-t border-border-subtle bg-surface-1/50">
              <button
                onClick={handleSaveSettings}
                className="w-full px-5 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl font-medium text-sm hover:from-blue-700 hover:to-indigo-700 transition-all shadow-lg shadow-blue-500/25 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                Save Settings
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ══════ Artifact Modal ══════ */}
      {artifactModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-labelledby="artifact-title">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => setArtifactModal(false)} aria-hidden="true" />
          <div className="relative w-full max-w-lg bg-surface-0 rounded-2xl shadow-2xl overflow-hidden">
            <div className="flex items-center justify-between p-5 border-b border-border-subtle">
              <h2 id="artifact-title" className="text-lg font-bold text-text-primary">Add Artifact</h2>
              <button
                onClick={() => setArtifactModal(false)}
                className="p-2 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                aria-label="Close"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleAddArtifact} className="p-5 space-y-4">
              {/* Type tabs */}
              <div className="flex gap-1 p-1 bg-surface-2 rounded-xl" role="tablist" aria-label="Artifact source">
                {[
                  { value: 'file', label: 'Upload File', icon: Upload },
                  { value: 'link', label: 'External Link', icon: LinkIcon },
                  { value: 'course_submission', label: 'From Course', icon: BookOpen },
                ].map((tab) => {
                  const TabIcon = tab.icon;
                  return (
                    <button
                      key={tab.value}
                      type="button"
                      onClick={() => setArtifactData((p) => ({ ...p, type: tab.value }))}
                      role="tab"
                      aria-selected={artifactData.type === tab.value}
                      className={`flex-1 flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium rounded-lg transition-all ${
                        artifactData.type === tab.value
                          ? 'bg-surface-0 text-text-primary shadow-sm'
                          : 'text-text-tertiary hover:text-text-secondary'
                      }`}
                    >
                      <TabIcon className="w-3.5 h-3.5" aria-hidden="true" />
                      {tab.label}
                    </button>
                  );
                })}
              </div>

              {/* Title */}
              <div>
                <label htmlFor="artifact-name" className="block text-sm font-semibold text-text-secondary mb-1">Title</label>
                <input
                  id="artifact-name"
                  type="text"
                  value={artifactData.title}
                  onChange={(e) => setArtifactData((p) => ({ ...p, title: e.target.value }))}
                  placeholder="Artifact title"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                  required
                />
              </div>

              {/* Description */}
              <div>
                <label htmlFor="artifact-desc" className="block text-sm font-semibold text-text-secondary mb-1">Description</label>
                <textarea
                  id="artifact-desc"
                  value={artifactData.description}
                  onChange={(e) => setArtifactData((p) => ({ ...p, description: e.target.value }))}
                  placeholder="Brief description of this work"
                  rows={3}
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm resize-y focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                />
              </div>

              {/* Type-specific field */}
              {artifactData.type === 'file' && (
                <div>
                  <label htmlFor="artifact-file" className="block text-sm font-semibold text-text-secondary mb-1">File</label>
                  <input
                    id="artifact-file"
                    type="file"
                    onChange={(e) => setArtifactData((p) => ({ ...p, file: e.target.files[0] || null }))}
                    className="w-full text-sm text-text-tertiary file:mr-3 file:px-4 file:py-2 file:rounded-xl file:border-0 file:bg-brand-50 file:text-brand-700 file:font-medium file:text-sm hover:file:bg-brand-100 file:cursor-pointer file:transition-colors"
                  />
                </div>
              )}

              {artifactData.type === 'link' && (
                <div>
                  <label htmlFor="artifact-url" className="block text-sm font-semibold text-text-secondary mb-1">URL</label>
                  <input
                    id="artifact-url"
                    type="url"
                    value={artifactData.url}
                    onChange={(e) => setArtifactData((p) => ({ ...p, url: e.target.value }))}
                    placeholder="https://..."
                    className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                    required
                  />
                </div>
              )}

              {artifactData.type === 'course_submission' && (
                <div>
                  <label htmlFor="artifact-submission" className="block text-sm font-semibold text-text-secondary mb-1">Submission ID</label>
                  <input
                    id="artifact-submission"
                    type="text"
                    value={artifactData.course_submission_id}
                    onChange={(e) => setArtifactData((p) => ({ ...p, course_submission_id: e.target.value }))}
                    placeholder="Enter course submission ID"
                    className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                  />
                </div>
              )}

              {/* Tags */}
              <div>
                <label htmlFor="artifact-tags" className="block text-sm font-semibold text-text-secondary mb-1">
                  Tags
                  <span className="text-xs text-text-disabled font-normal ml-1">(comma separated)</span>
                </label>
                <input
                  id="artifact-tags"
                  type="text"
                  value={artifactData.tags}
                  onChange={(e) => setArtifactData((p) => ({ ...p, tags: e.target.value }))}
                  placeholder="react, design, capstone"
                  className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                />
              </div>

              {/* Reflection (for course submissions) */}
              {artifactData.type === 'course_submission' && (
                <div>
                  <label htmlFor="artifact-reflection" className="flex items-center gap-2 text-sm font-semibold text-text-secondary mb-1">
                    <Lightbulb className="w-4 h-4 text-amber-500" aria-hidden="true" />
                    Reflection
                  </label>
                  <textarea
                    id="artifact-reflection"
                    value={artifactData.reflection}
                    onChange={(e) => setArtifactData((p) => ({ ...p, reflection: e.target.value }))}
                    placeholder="What did you learn from this project? How did it challenge you?"
                    rows={4}
                    className="w-full px-4 py-2.5 border border-border-default rounded-xl text-sm resize-y focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
                  />
                  <p className="text-xs text-text-disabled mt-1">Adding a reflection helps viewers understand the context and your growth.</p>
                </div>
              )}

              {/* Featured toggle */}
              <label className="flex items-center gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={artifactData.featured}
                  onChange={(e) => setArtifactData((p) => ({ ...p, featured: e.target.checked }))}
                  className="w-4 h-4 text-brand-600 border-border-strong rounded focus:ring-brand-500"
                />
                <div>
                  <span className="text-sm font-medium text-text-secondary">Featured artifact</span>
                  <p className="text-xs text-text-disabled">Featured artifacts are highlighted prominently in your portfolio</p>
                </div>
              </label>

              {/* Submit */}
              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setArtifactModal(false)}
                  className="px-5 py-2.5 text-sm font-medium text-text-secondary hover:bg-surface-2 rounded-xl transition-colors focus:outline-none focus:ring-2 focus:ring-gray-300"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-6 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-xl font-medium text-sm hover:from-blue-700 hover:to-indigo-700 transition-all shadow-lg shadow-blue-500/25 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
                >
                  Add Artifact
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};

export default PortfolioEditorPage;
