import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useParams } from 'react-router-dom';
import {
  Mail,
  Linkedin,
  Globe,
  ExternalLink,
  ChevronDown,
  ChevronUp,
  X,
  Copy,
  Check,
  Share2,
  MessageSquare,
  Send,
  Clock,
  Star,
  Tag,
  Award,
  Briefcase,
  GraduationCap,
  ArrowUp,
  Eye,
  Calendar,
  MapPin,
  Link as LinkIcon,
  FileText,
  Image,
  ChevronLeft,
  ChevronRight,
  User,
  Sparkles,
} from 'lucide-react';
import { api } from '../services/api';

/* ═══════════════════════════════════════════════════════
   THEME DEFINITIONS
   Each theme provides full class sets for the public page.
   ═══════════════════════════════════════════════════════ */
// User-facing portfolio theme presets — palette classes are intentional design tokens for the public-facing portfolio variants. Do not refactor to semantic tokens.
const THEMES = {
  clean_modern: {
    page: 'bg-white text-gray-900 font-sans',
    hero: 'bg-gradient-to-br from-blue-50 via-white to-indigo-50',
    heroText: 'text-gray-900',
    heroSub: 'text-gray-500',
    nav: 'bg-white/80 backdrop-blur-lg border-b border-gray-100',
    navLink: 'text-gray-600 hover:text-blue-600',
    navLinkActive: 'text-blue-600 font-semibold',
    section: 'bg-white',
    sectionAlt: 'bg-gray-50',
    heading: 'text-gray-900',
    body: 'text-gray-600',
    muted: 'text-gray-400',
    card: 'bg-white border border-gray-200 shadow-sm hover:shadow-xl hover:border-blue-200',
    cardTitle: 'text-gray-900',
    cardBody: 'text-gray-600',
    accent: 'text-blue-600',
    accentBg: 'bg-blue-600',
    accentBgLight: 'bg-blue-50',
    accentBorder: 'border-blue-500',
    badge: 'bg-blue-50 text-blue-700',
    skillBar: 'bg-blue-500',
    skillBg: 'bg-gray-100',
    timeline: 'border-blue-200',
    timelineDot: 'bg-blue-500 ring-4 ring-blue-100',
    button: 'bg-blue-600 text-white hover:bg-blue-700',
    buttonOutline: 'border border-gray-200 text-gray-600 hover:bg-gray-50',
    footer: 'bg-gray-50 border-t border-gray-100 text-gray-400',
    input: 'border-gray-200 bg-white text-gray-900 focus:ring-blue-500',
    lightbox: 'bg-black/80',
    avatarRing: 'ring-white',
  },
  creative_bold: {
    page: 'bg-gray-950 text-gray-100 font-sans',
    hero: 'bg-gradient-to-br from-gray-950 via-purple-950 to-gray-950',
    heroText: 'text-white',
    heroSub: 'text-purple-200',
    nav: 'bg-gray-950/80 backdrop-blur-lg border-b border-gray-800',
    navLink: 'text-gray-400 hover:text-pink-400',
    navLinkActive: 'text-pink-400 font-semibold',
    section: 'bg-gray-950',
    sectionAlt: 'bg-gray-900',
    heading: 'text-white',
    body: 'text-gray-300',
    muted: 'text-gray-500',
    card: 'bg-gray-900 border border-gray-800 hover:border-pink-500/50 hover:shadow-xl hover:shadow-pink-500/10',
    cardTitle: 'text-white',
    cardBody: 'text-gray-400',
    accent: 'text-pink-400',
    accentBg: 'bg-pink-500',
    accentBgLight: 'bg-pink-500/10',
    accentBorder: 'border-pink-500',
    badge: 'bg-pink-500/10 text-pink-400',
    skillBar: 'bg-gradient-to-r from-pink-500 to-purple-500',
    skillBg: 'bg-gray-800',
    timeline: 'border-purple-800',
    timelineDot: 'bg-pink-500 ring-4 ring-pink-500/20',
    button: 'bg-pink-500 text-white hover:bg-pink-600',
    buttonOutline: 'border border-gray-700 text-gray-300 hover:bg-gray-800',
    footer: 'bg-gray-950 border-t border-gray-800 text-gray-600',
    input: 'border-gray-700 bg-gray-900 text-gray-100 focus:ring-pink-500',
    lightbox: 'bg-black/90',
    avatarRing: 'ring-gray-950',
  },
  academic_classic: {
    page: 'bg-amber-50/30 text-gray-800 font-serif',
    hero: 'bg-gradient-to-br from-amber-50 via-orange-50/50 to-yellow-50',
    heroText: 'text-gray-900',
    heroSub: 'text-amber-700',
    nav: 'bg-amber-50/80 backdrop-blur-lg border-b border-amber-200/50',
    navLink: 'text-gray-600 hover:text-amber-700',
    navLinkActive: 'text-amber-800 font-semibold',
    section: 'bg-white/60',
    sectionAlt: 'bg-amber-50/40',
    heading: 'text-gray-900',
    body: 'text-gray-700',
    muted: 'text-gray-400',
    card: 'bg-white border border-amber-200/50 shadow-sm hover:shadow-lg hover:border-amber-300',
    cardTitle: 'text-gray-900',
    cardBody: 'text-gray-600',
    accent: 'text-amber-700',
    accentBg: 'bg-amber-600',
    accentBgLight: 'bg-amber-50',
    accentBorder: 'border-amber-500',
    badge: 'bg-amber-50 text-amber-800',
    skillBar: 'bg-amber-500',
    skillBg: 'bg-amber-100',
    timeline: 'border-amber-300',
    timelineDot: 'bg-amber-500 ring-4 ring-amber-100',
    button: 'bg-amber-700 text-white hover:bg-amber-800',
    buttonOutline: 'border border-amber-200 text-amber-700 hover:bg-amber-50',
    footer: 'bg-amber-50/50 border-t border-amber-200/50 text-gray-400',
    input: 'border-amber-200 bg-white text-gray-800 focus:ring-amber-500',
    lightbox: 'bg-black/80',
    avatarRing: 'ring-amber-50',
  },
  minimal_dark: {
    page: 'bg-gray-950 text-gray-200 font-mono',
    hero: 'bg-gray-950',
    heroText: 'text-gray-100',
    heroSub: 'text-gray-500',
    nav: 'bg-gray-950/80 backdrop-blur-lg border-b border-gray-800',
    navLink: 'text-gray-500 hover:text-gray-300',
    navLinkActive: 'text-gray-100 font-semibold',
    section: 'bg-gray-950',
    sectionAlt: 'bg-gray-900',
    heading: 'text-gray-100',
    body: 'text-gray-400',
    muted: 'text-gray-600',
    card: 'bg-gray-900 border border-gray-800 hover:border-gray-600',
    cardTitle: 'text-gray-200',
    cardBody: 'text-gray-500',
    accent: 'text-gray-300',
    accentBg: 'bg-gray-600',
    accentBgLight: 'bg-gray-800',
    accentBorder: 'border-gray-500',
    badge: 'bg-gray-800 text-gray-300',
    skillBar: 'bg-gray-400',
    skillBg: 'bg-gray-800',
    timeline: 'border-gray-800',
    timelineDot: 'bg-gray-400 ring-4 ring-gray-800',
    button: 'bg-gray-200 text-gray-900 hover:bg-white',
    buttonOutline: 'border border-gray-700 text-gray-400 hover:bg-gray-800',
    footer: 'bg-gray-950 border-t border-gray-800 text-gray-700',
    input: 'border-gray-700 bg-gray-900 text-gray-200 focus:ring-gray-500',
    lightbox: 'bg-black/90',
    avatarRing: 'ring-gray-950',
  },
  developer_portfolio: {
    page: 'bg-gray-950 text-emerald-100 font-mono',
    hero: 'bg-gradient-to-br from-gray-950 via-emerald-950/50 to-gray-950',
    heroText: 'text-emerald-50',
    heroSub: 'text-emerald-400',
    nav: 'bg-gray-950/80 backdrop-blur-lg border-b border-emerald-900/30',
    navLink: 'text-gray-500 hover:text-emerald-400',
    navLinkActive: 'text-emerald-400 font-semibold',
    section: 'bg-gray-950',
    sectionAlt: 'bg-gray-900/50',
    heading: 'text-emerald-50',
    body: 'text-gray-400',
    muted: 'text-gray-600',
    card: 'bg-gray-900/80 border border-emerald-900/30 hover:border-emerald-500/50 hover:shadow-lg hover:shadow-emerald-500/5',
    cardTitle: 'text-emerald-100',
    cardBody: 'text-gray-400',
    accent: 'text-emerald-400',
    accentBg: 'bg-emerald-500',
    accentBgLight: 'bg-emerald-500/10',
    accentBorder: 'border-emerald-500',
    badge: 'bg-emerald-500/10 text-emerald-400',
    skillBar: 'bg-emerald-500',
    skillBg: 'bg-gray-800',
    timeline: 'border-emerald-900',
    timelineDot: 'bg-emerald-500 ring-4 ring-emerald-500/20',
    button: 'bg-emerald-600 text-white hover:bg-emerald-500',
    buttonOutline: 'border border-emerald-800 text-emerald-400 hover:bg-emerald-900/30',
    footer: 'bg-gray-950 border-t border-emerald-900/30 text-gray-600',
    input: 'border-emerald-900 bg-gray-900 text-emerald-100 focus:ring-emerald-500',
    lightbox: 'bg-black/90',
    avatarRing: 'ring-gray-950',
  },
};

/* ───── helpers ───── */
const formatDate = (dateStr) => {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
};

const formatFullDate = (dateStr) => {
  if (!dateStr) return '';
  return new Date(dateStr).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
};

/* ═══════════════════════════════════════════════════════
   Component: PortfolioPublicPage
   ═══════════════════════════════════════════════════════ */
const PortfolioPublicPage = () => {
  const { slug } = useParams();

  /* ── State ── */
  const [portfolio, setPortfolio] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [activeSection, setActiveSection] = useState(null);

  /* Lightbox */
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [lightboxImages, setLightboxImages] = useState([]);
  const [lightboxIndex, setLightboxIndex] = useState(0);

  /* Artifact expand */
  const [expandedArtifact, setExpandedArtifact] = useState(null);

  /* Share */
  const [showShare, setShowShare] = useState(false);
  const [copied, setCopied] = useState(false);

  /* Comments */
  const [comments, setComments] = useState([]);
  const [commentText, setCommentText] = useState('');
  const [submittingComment, setSubmittingComment] = useState(false);
  const [showComments, setShowComments] = useState(false);

  /* Scroll to top */
  const [showScrollTop, setShowScrollTop] = useState(false);

  const sectionRefs = useRef({});
  const heroRef = useRef(null);

  /* ────────────────────────────────────
     Fetch portfolio
     ──────────────────────────────────── */
  useEffect(() => {
    const fetchPortfolio = async () => {
      try {
        const data = await api.getPublicPortfolio(slug);
        setPortfolio(data);
        if (data.sections?.length > 0) {
          const visible = data.sections.filter((s) => s.visible !== false).sort((a, b) => (a.order || 0) - (b.order || 0));
          if (visible.length > 0) setActiveSection(visible[0].id);
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchPortfolio();
  }, [slug]);

  /* Ping view counter */
  useEffect(() => {
    if (portfolio?.id) {
      api.recordPortfolioView(portfolio.id).catch(() => {});
    }
  }, [portfolio?.id]);

  /* Fetch comments */
  useEffect(() => {
    if (portfolio?.id) {
      api.getPortfolioComments(portfolio.id)
        .then((data) => setComments(Array.isArray(data) ? data : data.data || []))
        .catch(() => {});
    }
  }, [portfolio?.id]);

  /* Scroll spy */
  useEffect(() => {
    const handleScroll = () => {
      setShowScrollTop(window.scrollY > 600);

      /* Determine active section */
      const sections = portfolio?.sections?.filter((s) => s.visible !== false) || [];
      for (let i = sections.length - 1; i >= 0; i--) {
        const el = sectionRefs.current[sections[i].id];
        if (el) {
          const rect = el.getBoundingClientRect();
          if (rect.top <= 120) {
            setActiveSection(sections[i].id);
            break;
          }
        }
      }
    };
    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, [portfolio?.sections]);

  /* ────────────────────────────────────
     Smooth scroll to section
     ──────────────────────────────────── */
  const scrollToSection = (sectionId) => {
    const el = sectionRefs.current[sectionId];
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      setActiveSection(sectionId);
    }
  };

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  /* ────────────────────────────────────
     Share
     ──────────────────────────────────── */
  const handleCopyLink = () => {
    navigator.clipboard.writeText(window.location.href);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleShareLinkedIn = () => {
    window.open(
      `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(window.location.href)}`,
      '_blank',
      'noopener,noreferrer'
    );
  };

  const handleShareEmail = () => {
    const subject = encodeURIComponent(`Check out ${portfolio.title}`);
    const body = encodeURIComponent(`I wanted to share this portfolio with you: ${window.location.href}`);
    window.location.href = `mailto:?subject=${subject}&body=${body}`;
  };

  /* ────────────────────────────────────
     Comments
     ──────────────────────────────────── */
  const handleSubmitComment = async (e) => {
    e.preventDefault();
    if (!commentText.trim()) return;
    setSubmittingComment(true);
    try {
      const comment = await api.addPortfolioComment(portfolio.id, { content: commentText.trim() });
      setComments((prev) => [...prev, comment]);
      setCommentText('');
    } catch (err) {
      /* silently fail -- user might not be authenticated */
    } finally {
      setSubmittingComment(false);
    }
  };

  /* ────────────────────────────────────
     Lightbox
     ──────────────────────────────────── */
  const openLightbox = (images, index) => {
    setLightboxImages(images);
    setLightboxIndex(index);
    setLightboxOpen(true);
    document.body.style.overflow = 'hidden';
  };

  const closeLightbox = () => {
    setLightboxOpen(false);
    document.body.style.overflow = '';
  };

  const lightboxPrev = () => setLightboxIndex((i) => (i > 0 ? i - 1 : lightboxImages.length - 1));
  const lightboxNext = () => setLightboxIndex((i) => (i < lightboxImages.length - 1 ? i + 1 : 0));

  useEffect(() => {
    const handleKey = (e) => {
      if (!lightboxOpen) return;
      if (e.key === 'Escape') closeLightbox();
      if (e.key === 'ArrowLeft') lightboxPrev();
      if (e.key === 'ArrowRight') lightboxNext();
    };
    window.addEventListener('keydown', handleKey);
    return () => window.removeEventListener('keydown', handleKey);
  }, [lightboxOpen, lightboxImages.length]);

  /* ════════════════════════════════════
     Render
     ════════════════════════════════════ */

  /* Loading */
  if (loading) {
    return (
      <div className="min-h-screen bg-gray-950 flex items-center justify-center" role="status" aria-label="Loading portfolio">
        <div className="text-center">
          <div className="relative w-16 h-16 mx-auto mb-6">
            <div className="absolute inset-0 rounded-full border-4 border-gray-800" />
            <div className="absolute inset-0 rounded-full border-4 border-brand-500 border-t-transparent animate-spin" />
          </div>
          <p className="text-text-tertiary text-sm font-medium">Loading portfolio...</p>
        </div>
      </div>
    );
  }

  /* Error */
  if (error) {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center p-6">
        <div className="text-center max-w-md">
          <div className="w-20 h-20 bg-surface-2 rounded-3xl flex items-center justify-center mx-auto mb-6 rotate-3">
            <FileText className="w-10 h-10 text-gray-300" />
          </div>
          <h1 className="text-2xl font-bold text-text-primary mb-2">Portfolio Not Found</h1>
          <p className="text-text-tertiary mb-6">
            This portfolio may have been unpublished, moved, or the link may be incorrect.
          </p>
          <a href="/" className="text-brand-600 hover:underline font-medium text-sm">Go to Paper LMS</a>
        </div>
      </div>
    );
  }

  if (!portfolio) return null;

  const t = THEMES[portfolio.theme] || THEMES.clean_modern;
  const sections = (portfolio.sections || []).filter((s) => s.visible !== false).sort((a, b) => (a.order || 0) - (b.order || 0));
  const artifacts = portfolio.artifacts || [];

  /* ── Render section content by type ── */
  const renderSectionContent = (section) => {
    const sectionArtifacts = artifacts.filter((a) => a.section_id === section.id);

    switch (section.type) {

      /* ── Skills ── */
      case 'skills': {
        const skills = (section.skills || []).length > 0
          ? section.skills
          : (section.content || '').split('\n').filter(Boolean).map((line) => {
              const [name, level] = line.split(':').map((s) => s.trim());
              return { name, level: parseInt(level, 10) || 70 };
            });
        return (
          <div className={`grid ${section.layout === 'two-column' ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1'} gap-x-8 gap-y-4`}>
            {skills.length > 0 ? skills.map((skill, i) => (
              <div key={i}>
                <div className="flex items-center justify-between mb-1.5">
                  <span className={`text-sm font-medium ${t.heading}`}>{skill.name}</span>
                  <span className={`text-xs ${t.muted}`}>{skill.level}%</span>
                </div>
                <div className={`h-2 rounded-full ${t.skillBg} overflow-hidden`}>
                  <div
                    className={`h-full rounded-full ${t.skillBar} transition-all duration-1000 ease-out`}
                    style={{ width: `${skill.level}%` }}
                    role="progressbar"
                    aria-valuenow={skill.level}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-label={`${skill.name}: ${skill.level}%`}
                  />
                </div>
              </div>
            )) : (
              <div className={`flex flex-wrap gap-2`}>
                {(section.content || '').split(/[,\n]/).filter(Boolean).map((skill, i) => (
                  <span key={i} className={`px-4 py-2 rounded-full text-sm font-medium ${t.badge} transition-transform hover:scale-105`}>
                    {skill.trim()}
                  </span>
                ))}
              </div>
            )}
          </div>
        );
      }

      /* ── Experience / Education (Timeline) ── */
      case 'experience':
      case 'education': {
        const items = (section.items || []).length > 0
          ? section.items
          : (section.content || '').split('\n\n').filter(Boolean).map((block) => {
              const lines = block.split('\n');
              return { title: lines[0] || '', subtitle: lines[1] || '', description: lines.slice(2).join('\n') };
            });
        const Icon = section.type === 'education' ? GraduationCap : Briefcase;
        return (
          <div className={`relative pl-8 border-l-2 ${t.timeline} space-y-8`}>
            {items.map((item, i) => (
              <div key={i} className="relative">
                <div className={`absolute -left-[calc(1rem+5px)] top-1 w-3 h-3 rounded-full ${t.timelineDot}`} aria-hidden="true" />
                <div className="flex items-start gap-3">
                  <div className="flex-1">
                    <h4 className={`text-lg font-bold ${t.heading}`}>{item.title}</h4>
                    {item.subtitle && (
                      <p className={`text-sm mt-0.5 ${t.accent}`}>{item.subtitle}</p>
                    )}
                    {item.date && (
                      <p className={`text-xs mt-1 flex items-center gap-1 ${t.muted}`}>
                        <Calendar className="w-3 h-3" aria-hidden="true" />
                        {item.date}
                      </p>
                    )}
                    {item.location && (
                      <p className={`text-xs mt-0.5 flex items-center gap-1 ${t.muted}`}>
                        <MapPin className="w-3 h-3" aria-hidden="true" />
                        {item.location}
                      </p>
                    )}
                    {item.description && (
                      <p className={`text-sm mt-2 leading-relaxed ${t.body}`}>{item.description}</p>
                    )}
                  </div>
                </div>
              </div>
            ))}
            {items.length === 0 && section.content && (
              <p className={`text-sm leading-relaxed ${t.body} whitespace-pre-line`}>{section.content}</p>
            )}
          </div>
        );
      }

      /* ── Gallery ── */
      case 'gallery': {
        const images = sectionArtifacts.filter((a) => a.thumbnail_url || a.url);
        const imageUrls = images.map((a) => a.thumbnail_url || a.url);
        return (
          <div>
            {images.length > 0 ? (
              <div className={`grid ${
                section.layout === 'masonry' ? 'columns-2 md:columns-3 gap-4 space-y-4' :
                section.layout === 'grid' ? 'grid-cols-2 md:grid-cols-3 gap-4' :
                'grid-cols-1 md:grid-cols-2 gap-4'
              }`}>
                {images.map((img, i) => (
                  <button
                    key={img.id}
                    onClick={() => openLightbox(imageUrls, i)}
                    className={`${section.layout === 'masonry' ? 'break-inside-avoid' : ''} group relative rounded-xl overflow-hidden ${t.card} transition-all duration-300 focus:outline-none focus:ring-2 focus:ring-offset-2`}
                    aria-label={`View ${img.title}`}
                  >
                    <img
                      src={img.thumbnail_url || img.url}
                      alt={img.title || ''}
                      className="w-full h-auto object-cover group-hover:scale-105 transition-transform duration-500"
                      loading="lazy"
                    />
                    <div className="absolute inset-0 bg-gradient-to-t from-black/60 via-transparent to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex items-end p-4">
                      <div>
                        <p className="text-white text-sm font-semibold">{img.title}</p>
                        {img.description && <p className="text-white/70 text-xs mt-0.5">{img.description}</p>}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            ) : section.content ? (
              <p className={`text-sm leading-relaxed ${t.body} whitespace-pre-line`}>{section.content}</p>
            ) : null}
          </div>
        );
      }

      /* ── Projects (and other types) ── */
      default: {
        return (
          <div>
            {section.content && (
              <div className={`text-sm leading-relaxed ${t.body} whitespace-pre-line mb-6`}>
                {section.content}
              </div>
            )}

            {sectionArtifacts.length > 0 && (
              <div className={`grid ${
                section.layout === 'grid' ? 'grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5' :
                section.layout === 'two-column' ? 'grid-cols-1 md:grid-cols-2 gap-5' :
                section.layout === 'masonry' ? 'columns-1 md:columns-2 gap-5 space-y-5' :
                'grid-cols-1 gap-5'
              }`}>
                {sectionArtifacts.map((artifact) => (
                  <article
                    key={artifact.id}
                    className={`${section.layout === 'masonry' ? 'break-inside-avoid' : ''} group rounded-xl ${t.card} overflow-hidden transition-all duration-300 cursor-pointer`}
                    onClick={() => setExpandedArtifact(expandedArtifact === artifact.id ? null : artifact.id)}
                    role="button"
                    aria-expanded={expandedArtifact === artifact.id}
                    tabIndex={0}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setExpandedArtifact(expandedArtifact === artifact.id ? null : artifact.id); } }}
                  >
                    {/* Thumbnail */}
                    {artifact.thumbnail_url && (
                      <div className="overflow-hidden">
                        <img
                          src={artifact.thumbnail_url}
                          alt=""
                          className="w-full h-48 object-cover group-hover:scale-105 transition-transform duration-500"
                          loading="lazy"
                        />
                      </div>
                    )}
                    <div className="p-5">
                      <div className="flex items-start justify-between gap-2">
                        <h4 className={`font-bold ${t.cardTitle}`}>
                          {artifact.featured && <Star className="w-4 h-4 inline-block mr-1.5 text-amber-500" aria-label="Featured" />}
                          {artifact.title}
                        </h4>
                        {artifact.url && (
                          <a
                            href={artifact.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className={`shrink-0 p-1 rounded-lg ${t.muted} hover:${t.accent} transition-colors`}
                            onClick={(e) => e.stopPropagation()}
                            aria-label={`Open ${artifact.title} externally`}
                          >
                            <ExternalLink className="w-4 h-4" />
                          </a>
                        )}
                      </div>
                      {artifact.description && (
                        <p className={`text-sm mt-2 ${expandedArtifact === artifact.id ? '' : 'line-clamp-2'} ${t.cardBody}`}>
                          {artifact.description}
                        </p>
                      )}

                      {/* Tags */}
                      {artifact.tags?.length > 0 && (
                        <div className="flex flex-wrap gap-1.5 mt-3">
                          {artifact.tags.map((tag) => (
                            <span key={tag} className={`px-2 py-0.5 rounded-full text-xs font-medium ${t.badge}`}>{tag}</span>
                          ))}
                        </div>
                      )}

                      {/* Expanded: reflection */}
                      {expandedArtifact === artifact.id && artifact.reflection && (
                        <div className={`mt-4 pt-4 border-t ${t.timeline}`}>
                          <p className={`text-xs uppercase tracking-wide font-semibold ${t.accent} mb-2 flex items-center gap-1`}>
                            <Sparkles className="w-3.5 h-3.5" aria-hidden="true" />
                            Reflection
                          </p>
                          <p className={`text-sm leading-relaxed italic ${t.body}`}>{artifact.reflection}</p>
                        </div>
                      )}
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        );
      }
    }
  };

  /* ════════════════════════════════════
     Page Render
     ════════════════════════════════════ */
  return (
    <div className={`min-h-screen ${t.page}`}>

      {/* Custom CSS */}
      {portfolio.custom_css && (
        <style>{portfolio.custom_css}</style>
      )}

      {/* ═══════ Sticky Navigation ═══════ */}
      {sections.length > 1 && (
        <nav className={`sticky top-0 z-40 ${t.nav}`} role="navigation" aria-label="Portfolio sections">
          <div className="max-w-5xl mx-auto px-6">
            <div className="flex items-center gap-1 overflow-x-auto py-3 scrollbar-hide">
              {sections.map((section) => (
                <button
                  key={section.id}
                  onClick={() => scrollToSection(section.id)}
                  className={`px-4 py-1.5 rounded-full text-sm whitespace-nowrap transition-all ${
                    activeSection === section.id ? t.navLinkActive : t.navLink
                  }`}
                >
                  {section.title}
                </button>
              ))}
              <div className="flex-1" />
              {/* Share button in nav */}
              <button
                onClick={() => setShowShare(!showShare)}
                className={`p-2 rounded-full ${t.navLink} transition-colors focus:outline-none`}
                aria-label="Share portfolio"
              >
                <Share2 className="w-4 h-4" />
              </button>
            </div>
          </div>
        </nav>
      )}

      {/* ═══════ Hero Section ═══════ */}
      <section ref={heroRef} className={`relative ${t.hero} overflow-hidden`} aria-label="Portfolio header">
        {/* Cover image */}
        {portfolio.cover_image_url && (
          <div className="absolute inset-0">
            <img src={portfolio.cover_image_url} alt="" className="w-full h-full object-cover" />
            <div className="absolute inset-0 bg-gradient-to-b from-black/40 via-black/20 to-black/60" />
          </div>
        )}

        <div className="relative max-w-5xl mx-auto px-6 py-20 md:py-28 text-center">
          {/* Avatar */}
          <div className={`w-28 h-28 rounded-full ring-4 ${t.avatarRing} shadow-2xl mx-auto mb-6 overflow-hidden bg-border-default`}>
            {portfolio.avatar_url ? (
              <img src={portfolio.avatar_url} alt={portfolio.user_name || 'Portfolio author'} className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full flex items-center justify-center">
                <User className="w-14 h-14 text-text-disabled" aria-hidden="true" />
              </div>
            )}
          </div>

          {/* Name + tagline */}
          <h1 className={`text-4xl md:text-5xl font-bold tracking-tight mb-3 ${portfolio.cover_image_url ? 'text-white' : t.heroText}`}>
            {portfolio.title}
          </h1>
          {portfolio.tagline && (
            <p className={`text-lg md:text-xl max-w-2xl mx-auto ${portfolio.cover_image_url ? 'text-white/80' : t.heroSub}`}>
              {portfolio.tagline}
            </p>
          )}

          {/* Contact links */}
          <div className="flex items-center justify-center gap-4 mt-8">
            {portfolio.contact_email && (
              <a
                href={`mailto:${portfolio.contact_email}`}
                className={`p-3 rounded-full ${portfolio.cover_image_url ? 'bg-surface-0/10 text-white hover:bg-surface-0/20' : `${t.accentBgLight} ${t.accent}`} transition-all hover:scale-110`}
                aria-label="Send email"
              >
                <Mail className="w-5 h-5" />
              </a>
            )}
            {portfolio.linkedin_url && (
              <a
                href={portfolio.linkedin_url}
                target="_blank"
                rel="noopener noreferrer"
                className={`p-3 rounded-full ${portfolio.cover_image_url ? 'bg-surface-0/10 text-white hover:bg-surface-0/20' : `${t.accentBgLight} ${t.accent}`} transition-all hover:scale-110`}
                aria-label="LinkedIn profile"
              >
                <Linkedin className="w-5 h-5" />
              </a>
            )}
            {portfolio.website_url && (
              <a
                href={portfolio.website_url}
                target="_blank"
                rel="noopener noreferrer"
                className={`p-3 rounded-full ${portfolio.cover_image_url ? 'bg-surface-0/10 text-white hover:bg-surface-0/20' : `${t.accentBgLight} ${t.accent}`} transition-all hover:scale-110`}
                aria-label="Personal website"
              >
                <Globe className="w-5 h-5" />
              </a>
            )}
          </div>
        </div>
      </section>

      {/* ═══════ Sections ═══════ */}
      <main id="main-content" role="main">
        {sections.map((section, idx) => (
          <section
            key={section.id}
            ref={(el) => { sectionRefs.current[section.id] = el; }}
            className={`py-16 md:py-20 ${idx % 2 === 0 ? t.section : t.sectionAlt}`}
            aria-labelledby={`section-heading-${section.id}`}
            id={`section-${section.id}`}
          >
            <div className="max-w-5xl mx-auto px-6">
              <h2
                id={`section-heading-${section.id}`}
                className={`text-2xl md:text-3xl font-bold mb-8 ${t.heading}`}
              >
                {section.title}
              </h2>
              {renderSectionContent(section)}
            </div>
          </section>
        ))}
      </main>

      {/* ═══════ Comments Section ═══════ */}
      <section className={`py-12 ${t.sectionAlt}`} aria-labelledby="comments-heading">
        <div className="max-w-3xl mx-auto px-6">
          <button
            onClick={() => setShowComments(!showComments)}
            className={`flex items-center gap-2 text-sm font-semibold ${t.accent} mb-6`}
            aria-expanded={showComments}
          >
            <MessageSquare className="w-5 h-5" aria-hidden="true" />
            Feedback ({comments.length})
            {showComments ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </button>

          {showComments && (
            <div className="space-y-6">
              {/* Comment list */}
              {comments.length > 0 ? (
                <div className="space-y-4">
                  {comments.map((comment) => (
                    <div key={comment.id} className={`${t.card} rounded-xl p-4`}>
                      <div className="flex items-center gap-3 mb-2">
                        <div className={`w-8 h-8 rounded-full ${t.accentBgLight} flex items-center justify-center`}>
                          <User className={`w-4 h-4 ${t.accent}`} aria-hidden="true" />
                        </div>
                        <div>
                          <p className={`text-sm font-semibold ${t.heading}`}>{comment.user_name || 'Anonymous'}</p>
                          <p className={`text-xs ${t.muted}`}>{formatFullDate(comment.created_at)}</p>
                        </div>
                      </div>
                      <p className={`text-sm leading-relaxed ${t.body}`}>{comment.content}</p>
                    </div>
                  ))}
                </div>
              ) : (
                <p className={`text-sm ${t.muted}`}>No feedback yet. Be the first to leave a comment.</p>
              )}

              {/* Comment form */}
              <form onSubmit={handleSubmitComment} className="flex gap-3">
                <input
                  type="text"
                  value={commentText}
                  onChange={(e) => setCommentText(e.target.value)}
                  placeholder="Leave feedback (sign in required)..."
                  className={`flex-1 px-4 py-2.5 border rounded-xl text-sm ${t.input} focus:outline-none focus:ring-2 transition-shadow`}
                  aria-label="Write a comment"
                />
                <button
                  type="submit"
                  disabled={submittingComment || !commentText.trim()}
                  className={`px-5 py-2.5 rounded-xl text-sm font-medium ${t.button} transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-offset-2`}
                  aria-label="Submit comment"
                >
                  <Send className="w-4 h-4" aria-hidden="true" />
                </button>
              </form>
            </div>
          )}
        </div>
      </section>

      {/* ═══════ Footer ═══════ */}
      <footer className={`py-8 text-center ${t.footer}`} role="contentinfo">
        <div className="max-w-5xl mx-auto px-6">
          <div className="flex items-center justify-center gap-4 mb-4">
            {/* Share buttons */}
            <button
              onClick={handleCopyLink}
              className={`p-2 rounded-lg ${t.buttonOutline} transition-colors text-sm focus:outline-none`}
              aria-label={copied ? 'Link copied' : 'Copy portfolio link'}
            >
              {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
            </button>
            <button
              onClick={handleShareLinkedIn}
              className={`p-2 rounded-lg ${t.buttonOutline} transition-colors text-sm focus:outline-none`}
              aria-label="Share on LinkedIn"
            >
              <Linkedin className="w-4 h-4" />
            </button>
            <button
              onClick={handleShareEmail}
              className={`p-2 rounded-lg ${t.buttonOutline} transition-colors text-sm focus:outline-none`}
              aria-label="Share via email"
            >
              <Mail className="w-4 h-4" />
            </button>
          </div>
          <p className="text-xs">
            Powered by{' '}
            <a href="/" className={`${t.accent} hover:underline font-medium`}>
              Paper LMS
            </a>
          </p>
        </div>
      </footer>

      {/* ═══════ Share Modal ═══════ */}
      {showShare && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true" aria-label="Share portfolio">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => setShowShare(false)} aria-hidden="true" />
          <div className="relative w-full max-w-sm bg-surface-0 rounded-2xl shadow-2xl p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-lg font-bold text-text-primary">Share Portfolio</h3>
              <button onClick={() => setShowShare(false)} className="p-2 rounded-lg text-text-disabled hover:text-text-secondary hover:bg-surface-2 focus:outline-none" aria-label="Close">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-3">
              {/* Copy link */}
              <div className="flex items-center gap-2 p-3 bg-surface-1 rounded-xl">
                <input
                  type="text"
                  value={window.location.href}
                  readOnly
                  className="flex-1 bg-transparent text-sm text-text-secondary outline-none"
                  aria-label="Portfolio URL"
                />
                <button
                  onClick={handleCopyLink}
                  className="px-3 py-1.5 bg-brand-600 text-white rounded-lg text-xs font-medium hover:bg-brand-700 transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>

              {/* Share buttons */}
              <div className="grid grid-cols-2 gap-2">
                <button
                  onClick={handleShareLinkedIn}
                  className="flex items-center justify-center gap-2 p-3 bg-[#0077B5] text-white rounded-xl text-sm font-medium hover:bg-[#006397] transition-colors focus:outline-none focus:ring-2 focus:ring-brand-500"
                >
                  <Linkedin className="w-4 h-4" /> LinkedIn
                </button>
                <button
                  onClick={handleShareEmail}
                  className="flex items-center justify-center gap-2 p-3 bg-gray-800 text-white rounded-xl text-sm font-medium hover:bg-gray-700 transition-colors focus:outline-none focus:ring-2 focus:ring-gray-500"
                >
                  <Mail className="w-4 h-4" /> Email
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ═══════ Lightbox ═══════ */}
      {lightboxOpen && lightboxImages.length > 0 && (
        <div
          className={`fixed inset-0 z-50 ${t.lightbox} flex items-center justify-center`}
          role="dialog"
          aria-modal="true"
          aria-label="Image viewer"
        >
          <button
            onClick={closeLightbox}
            className="absolute top-4 right-4 p-3 text-white/70 hover:text-white bg-black/30 rounded-full transition-colors z-10 focus:outline-none focus:ring-2 focus:ring-white"
            aria-label="Close image viewer"
          >
            <X className="w-6 h-6" />
          </button>

          {lightboxImages.length > 1 && (
            <>
              <button
                onClick={lightboxPrev}
                className="absolute left-4 top-1/2 -translate-y-1/2 p-3 text-white/70 hover:text-white bg-black/30 rounded-full transition-colors z-10 focus:outline-none focus:ring-2 focus:ring-white"
                aria-label="Previous image"
              >
                <ChevronLeft className="w-6 h-6" />
              </button>
              <button
                onClick={lightboxNext}
                className="absolute right-4 top-1/2 -translate-y-1/2 p-3 text-white/70 hover:text-white bg-black/30 rounded-full transition-colors z-10 focus:outline-none focus:ring-2 focus:ring-white"
                aria-label="Next image"
              >
                <ChevronRight className="w-6 h-6" />
              </button>
            </>
          )}

          <img
            src={lightboxImages[lightboxIndex]}
            alt=""
            className="max-w-[90vw] max-h-[85vh] object-contain rounded-lg shadow-2xl"
          />

          {lightboxImages.length > 1 && (
            <div className="absolute bottom-6 left-1/2 -translate-x-1/2 flex gap-2">
              {lightboxImages.map((_, i) => (
                <button
                  key={i}
                  onClick={() => setLightboxIndex(i)}
                  className={`w-2 h-2 rounded-full transition-all ${
                    i === lightboxIndex ? 'bg-surface-0 scale-125' : 'bg-surface-0/40 hover:bg-surface-0/60'
                  }`}
                  aria-label={`Go to image ${i + 1}`}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* ═══════ Expanded Artifact Modal ═══════ */}
      {expandedArtifact && (() => {
        const artifact = artifacts.find((a) => a.id === expandedArtifact);
        if (!artifact) return null;
        return null; /* Expansion handled inline on the card itself */
      })()}

      {/* ═══════ Scroll to Top ═══════ */}
      {showScrollTop && (
        <button
          onClick={scrollToTop}
          className={`fixed bottom-6 right-6 p-3 rounded-full ${t.button} shadow-xl transition-all hover:scale-110 z-30 focus:outline-none focus:ring-2 focus:ring-offset-2`}
          aria-label="Scroll to top"
        >
          <ArrowUp className="w-5 h-5" />
        </button>
      )}
    </div>
  );
};

export default PortfolioPublicPage;
