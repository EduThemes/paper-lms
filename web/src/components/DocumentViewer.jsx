import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import {
  Highlighter,
  MessageSquare,
  Strikethrough,
  Pencil,
  MapPin,
  Check,
  X,
  ChevronDown,
  ChevronUp,
  Filter,
  Eye,
  EyeOff,
  Clock,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import { sanitizeHTML } from './RichContentViewer';

const ANNOTATION_TYPES = {
  highlight: { label: 'Highlight', icon: Highlighter, color: '#FFFF00' },
  comment: { label: 'Comment', icon: MessageSquare, color: '#3B82F6' },
  strikethrough: { label: 'Strikethrough', icon: Strikethrough, color: '#EF4444' },
  freehand: { label: 'Freehand', icon: Pencil, color: '#8B5CF6' },
  point: { label: 'Point', icon: MapPin, color: '#10B981' },
};

const DEFAULT_COLORS = [
  '#FFFF00', '#FF9800', '#F44336', '#E91E63',
  '#9C27B0', '#3F51B5', '#2196F3', '#00BCD4',
  '#4CAF50', '#8BC34A', '#795548', '#607D8B',
];

const formatTimestamp = (dateStr) => {
  if (!dateStr) return '';
  const d = new Date(dateStr);
  const now = new Date();
  const diff = now - d;
  if (diff < 60000) return 'just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  return d.toLocaleDateString();
};

const DocumentViewer = ({
  submissionId,
  courseId,
  assignmentId,
  userId,
  readOnly = false,
  documentContent = '',
  documentTitle = '',
}) => {
  const { user } = useAuth();
  const [annotations, setAnnotations] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTool, setActiveTool] = useState(null);
  const [activeColor, setActiveColor] = useState('#FFFF00');
  const [showColorPicker, setShowColorPicker] = useState(false);
  const [selectedAnnotation, setSelectedAnnotation] = useState(null);
  const [replyText, setReplyText] = useState('');
  const [commentText, setCommentText] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const [filterType, setFilterType] = useState('all');
  const [filterAuthor, setFilterAuthor] = useState('all');
  const [showResolved, setShowResolved] = useState(true);
  const [isDrawing, setIsDrawing] = useState(false);
  const [currentPath, setCurrentPath] = useState('');
  const [pendingAnnotation, setPendingAnnotation] = useState(null);

  const contentRef = useRef(null);
  const svgRef = useRef(null);
  const sidebarRef = useRef(null);

  // Fetch annotations
  const fetchAnnotations = useCallback(async () => {
    try {
      setLoading(true);
      const data = await api.getAnnotations(courseId, assignmentId, userId);
      setAnnotations(data || []);
    } catch (err) {
      console.error('Failed to fetch annotations:', err);
    } finally {
      setLoading(false);
    }
  }, [courseId, assignmentId, userId]);

  useEffect(() => {
    fetchAnnotations();
  }, [fetchAnnotations]);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        setActiveTool(null);
        setPendingAnnotation(null);
        setIsDrawing(false);
        setCurrentPath('');
      }
      if (e.key === 'Enter' && pendingAnnotation) {
        handleSaveAnnotation();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [pendingAnnotation]);

  // Handle text selection for highlight/strikethrough
  const handleTextSelection = useCallback(() => {
    if (readOnly || !activeTool || !['highlight', 'strikethrough'].includes(activeTool)) return;
    const selection = window.getSelection();
    if (!selection || selection.isCollapsed) return;

    const range = selection.getRangeAt(0);
    const contentEl = contentRef.current;
    if (!contentEl || !contentEl.contains(range.commonAncestorContainer)) return;

    // Calculate character positions relative to the content container
    const preSelectionRange = document.createRange();
    preSelectionRange.selectNodeContents(contentEl);
    preSelectionRange.setEnd(range.startContainer, range.startOffset);
    const start = preSelectionRange.toString().length;
    const end = start + selection.toString().length;

    setPendingAnnotation({
      annotation_type: activeTool,
      color: activeColor,
      content: '',
      page_number: 1,
      selection_start: start,
      selection_end: end,
      selected_text: selection.toString(),
    });
  }, [activeTool, activeColor, readOnly]);

  useEffect(() => {
    document.addEventListener('mouseup', handleTextSelection);
    return () => document.removeEventListener('mouseup', handleTextSelection);
  }, [handleTextSelection]);

  // Handle click for comment/point annotation
  const handleContentClick = useCallback((e) => {
    if (readOnly || !activeTool || !['comment', 'point'].includes(activeTool)) return;
    const contentEl = contentRef.current;
    if (!contentEl) return;
    const rect = contentEl.getBoundingClientRect();
    const x = ((e.clientX - rect.left) / rect.width) * 100;
    const y = ((e.clientY - rect.top) / rect.height) * 100;

    setPendingAnnotation({
      annotation_type: activeTool,
      color: activeColor,
      content: '',
      page_number: 1,
      x,
      y,
      width: 0,
      height: 0,
    });
  }, [activeTool, activeColor, readOnly]);

  // Handle freehand drawing
  const handleDrawStart = useCallback((e) => {
    if (readOnly || activeTool !== 'freehand') return;
    const svgEl = svgRef.current;
    if (!svgEl) return;
    const rect = svgEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    setIsDrawing(true);
    setCurrentPath(`M ${x} ${y}`);
  }, [activeTool, readOnly]);

  const handleDrawMove = useCallback((e) => {
    if (!isDrawing) return;
    const svgEl = svgRef.current;
    if (!svgEl) return;
    const rect = svgEl.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    setCurrentPath(prev => `${prev} L ${x} ${y}`);
  }, [isDrawing]);

  const handleDrawEnd = useCallback(() => {
    if (!isDrawing || !currentPath) return;
    setIsDrawing(false);
    setPendingAnnotation({
      annotation_type: 'freehand',
      color: activeColor,
      content: '',
      page_number: 1,
      path_data: currentPath,
      x: 0,
      y: 0,
      width: 0,
      height: 0,
    });
  }, [isDrawing, currentPath, activeColor]);

  // Save annotation
  const handleSaveAnnotation = async () => {
    if (!pendingAnnotation) return;
    try {
      const annotationData = {
        ...pendingAnnotation,
        content: pendingAnnotation.annotation_type === 'comment' || pendingAnnotation.annotation_type === 'point'
          ? commentText
          : pendingAnnotation.content || '',
      };
      const newAnnotation = await api.createAnnotation(courseId, assignmentId, userId, annotationData);
      setAnnotations(prev => [...prev, newAnnotation]);
      setPendingAnnotation(null);
      setCommentText('');
      setCurrentPath('');
      setActiveTool(null);
    } catch (err) {
      console.error('Failed to save annotation:', err);
    }
  };

  // Delete annotation
  const handleDeleteAnnotation = async (annotationId) => {
    try {
      await api.deleteAnnotation(annotationId, courseId);
      setAnnotations(prev => prev.filter(a => a.id !== annotationId));
      if (selectedAnnotation?.id === annotationId) setSelectedAnnotation(null);
    } catch (err) {
      console.error('Failed to delete annotation:', err);
    }
  };

  // Resolve/unresolve annotation
  const handleToggleResolve = async (annotation) => {
    try {
      if (annotation.workflow_state === 'resolved') {
        await api.unresolveAnnotation(annotation.id);
        setAnnotations(prev => prev.map(a =>
          a.id === annotation.id ? { ...a, workflow_state: 'active', resolved_at: null } : a
        ));
      } else {
        await api.resolveAnnotation(annotation.id);
        setAnnotations(prev => prev.map(a =>
          a.id === annotation.id ? { ...a, workflow_state: 'resolved', resolved_at: new Date().toISOString() } : a
        ));
      }
    } catch (err) {
      console.error('Failed to toggle resolve:', err);
    }
  };

  // Reply to annotation
  const handleReply = async (annotationId) => {
    if (!replyText.trim()) return;
    try {
      const reply = await api.replyToAnnotation(annotationId, replyText, courseId);
      setAnnotations(prev => prev.map(a =>
        a.id === annotationId
          ? { ...a, replies: [...(a.replies || []), reply] }
          : a
      ));
      setReplyText('');
    } catch (err) {
      console.error('Failed to reply:', err);
    }
  };

  // Scroll to annotation in content
  const scrollToAnnotation = (annotation) => {
    setSelectedAnnotation(annotation);
    // For text annotations, scroll to the character position
    if (contentRef.current && (annotation.selection_start || annotation.x)) {
      contentRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  };

  // Filter annotations
  const filteredAnnotations = useMemo(() => {
    return annotations.filter(a => {
      if (filterType !== 'all' && a.annotation_type !== filterType) return false;
      if (filterAuthor !== 'all' && String(a.user_id) !== filterAuthor) return false;
      if (!showResolved && a.workflow_state === 'resolved') return false;
      return true;
    });
  }, [annotations, filterType, filterAuthor, showResolved]);

  // Get unique authors
  const authors = useMemo(() => {
    const authorMap = {};
    annotations.forEach(a => {
      if (a.user) {
        authorMap[a.user_id] = a.user.name;
      } else {
        authorMap[a.user_id] = `User ${a.user_id}`;
      }
    });
    return authorMap;
  }, [annotations]);

  // Render annotation overlays on the document
  const renderAnnotationOverlays = () => {
    return annotations.map(annotation => {
      if (annotation.workflow_state === 'deleted') return null;
      if (!showResolved && annotation.workflow_state === 'resolved') return null;

      const isSelected = selectedAnnotation?.id === annotation.id;
      const opacity = annotation.workflow_state === 'resolved' ? 0.4 : 0.6;

      if (annotation.annotation_type === 'point' || annotation.annotation_type === 'comment') {
        return (
          <div
            key={annotation.id}
            className={`absolute cursor-pointer transition-all ${isSelected ? 'z-20 scale-125' : 'z-10'}`}
            style={{
              left: `${annotation.x}%`,
              top: `${annotation.y}%`,
              transform: 'translate(-50%, -50%)',
            }}
            onClick={() => setSelectedAnnotation(annotation)}
            title={annotation.content}
          >
            <div
              className="w-6 h-6 rounded-full flex items-center justify-center text-white text-xs font-bold shadow-md"
              style={{ backgroundColor: annotation.color || '#3B82F6', opacity }}
            >
              {annotation.annotation_type === 'comment' ? (
                <MessageSquare className="w-3 h-3" />
              ) : (
                <MapPin className="w-3 h-3" />
              )}
            </div>
            {isSelected && annotation.content && (
              <div className="absolute left-8 top-0 bg-surface-0 border border-border-default rounded-lg shadow-lg p-3 min-w-48 max-w-72 z-30">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs font-medium text-text-secondary">
                    {annotation.user?.name || `User ${annotation.user_id}`}
                  </span>
                  <span className="text-xs text-text-disabled">{formatTimestamp(annotation.created_at)}</span>
                </div>
                <p className="text-sm text-text-primary">{annotation.content}</p>
              </div>
            )}
          </div>
        );
      }

      return null;
    });
  };

  // Render freehand SVG overlays
  const renderFreehandOverlays = () => {
    return annotations
      .filter(a => a.annotation_type === 'freehand' && a.workflow_state !== 'deleted')
      .map(annotation => {
        if (!showResolved && annotation.workflow_state === 'resolved') return null;
        const isSelected = selectedAnnotation?.id === annotation.id;
        return (
          <path
            key={annotation.id}
            d={annotation.path_data}
            fill="none"
            stroke={annotation.color || '#8B5CF6'}
            strokeWidth={isSelected ? 3 : 2}
            strokeLinecap="round"
            strokeLinejoin="round"
            opacity={annotation.workflow_state === 'resolved' ? 0.3 : 0.7}
            className="cursor-pointer"
            onClick={() => setSelectedAnnotation(annotation)}
          />
        );
      });
  };

  // Render text highlights inline
  const renderHighlightedContent = () => {
    if (!documentContent) {
      return <p className="text-text-disabled italic text-center py-8">No document content available</p>;
    }

    // For simplicity, we render the document content as HTML with highlight markers
    // A production implementation would use a more sophisticated text node walker
    let content = documentContent;

    // Sort text annotations by position (reverse order to not invalidate positions)
    const textAnnotations = annotations
      .filter(a => ['highlight', 'strikethrough'].includes(a.annotation_type) && a.workflow_state !== 'deleted')
      .filter(a => showResolved || a.workflow_state !== 'resolved')
      .sort((a, b) => b.selection_start - a.selection_start);

    // We'll render the raw HTML and let CSS handle the visual styling
    // since inserting markers into HTML is complex
    return (
      <div
        className="prose max-w-none text-text-secondary leading-relaxed"
        dangerouslySetInnerHTML={{ __html: sanitizeHTML(content) }}
      />
    );
  };

  // Toolbar component
  const renderToolbar = () => {
    if (readOnly) return null;

    return (
      <div className="flex items-center gap-1 p-2 bg-surface-1 border-b border-border-default flex-wrap">
        {Object.entries(ANNOTATION_TYPES).map(([type, config]) => {
          const Icon = config.icon;
          const isActive = activeTool === type;
          return (
            <button
              key={type}
              onClick={() => {
                setActiveTool(isActive ? null : type);
                setPendingAnnotation(null);
                setCurrentPath('');
              }}
              className={`flex items-center gap-1 px-2.5 py-1.5 rounded text-xs font-medium transition-colors ${
                isActive
                  ? 'bg-brand-600 text-white shadow-sm'
                  : 'bg-surface-0 text-text-secondary hover:bg-surface-2 border border-border-default'
              }`}
              title={`${config.label} (${type})`}
            >
              <Icon className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">{config.label}</span>
            </button>
          );
        })}

        <div className="w-px h-6 bg-gray-300 mx-1" />

        {/* Color picker */}
        <div className="relative">
          <button
            onClick={() => setShowColorPicker(!showColorPicker)}
            className="flex items-center gap-1 px-2 py-1.5 rounded text-xs font-medium bg-surface-0 text-text-secondary hover:bg-surface-2 border border-border-default"
          >
            <div
              className="w-4 h-4 rounded-sm border border-border-strong"
              style={{ backgroundColor: activeColor }}
            />
            <ChevronDown className="w-3 h-3" />
          </button>
          {showColorPicker && (
            <div className="absolute top-full left-0 mt-1 bg-surface-0 border border-border-default rounded-lg shadow-lg p-2 z-50 grid grid-cols-4 gap-1">
              {DEFAULT_COLORS.map(color => (
                <button
                  key={color}
                  onClick={() => {
                    setActiveColor(color);
                    setShowColorPicker(false);
                  }}
                  className={`w-6 h-6 rounded-sm border-2 transition-all ${
                    activeColor === color ? 'border-brand-500 scale-110' : 'border-border-default hover:border-border-strong'
                  }`}
                  style={{ backgroundColor: color }}
                />
              ))}
            </div>
          )}
        </div>

        {activeTool && (
          <>
            <div className="w-px h-6 bg-gray-300 mx-1" />
            <span className="text-xs text-brand-600 font-medium">
              {activeTool === 'highlight' || activeTool === 'strikethrough'
                ? 'Select text to annotate'
                : activeTool === 'freehand'
                ? 'Draw on document'
                : 'Click on document to place'}
            </span>
            <button
              onClick={() => {
                setActiveTool(null);
                setPendingAnnotation(null);
              }}
              className="text-xs text-text-tertiary hover:text-accent-danger ml-1"
              title="Cancel (Esc)"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </>
        )}
      </div>
    );
  };

  // Pending annotation dialog
  const renderPendingAnnotation = () => {
    if (!pendingAnnotation) return null;

    return (
      <div className="absolute bottom-4 left-4 right-4 bg-surface-0 border border-border-default rounded-lg shadow-xl p-4 z-50">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-text-secondary">
            {ANNOTATION_TYPES[pendingAnnotation.annotation_type]?.label || 'Annotation'}
          </span>
          <button
            onClick={() => {
              setPendingAnnotation(null);
              setCommentText('');
            }}
            className="text-text-disabled hover:text-text-secondary"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        {pendingAnnotation.selected_text && (
          <div className="text-xs text-text-tertiary bg-surface-1 p-2 rounded mb-2 italic truncate">
            &ldquo;{pendingAnnotation.selected_text}&rdquo;
          </div>
        )}
        {(pendingAnnotation.annotation_type === 'comment' || pendingAnnotation.annotation_type === 'point') && (
          <textarea
            value={commentText}
            onChange={(e) => setCommentText(e.target.value)}
            placeholder="Add a comment..."
            className="w-full border border-border-strong rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 mb-2 resize-none"
            rows={2}
            autoFocus
          />
        )}
        <div className="flex justify-end gap-2">
          <button
            onClick={() => {
              setPendingAnnotation(null);
              setCommentText('');
            }}
            className="px-3 py-1.5 text-xs text-text-secondary hover:bg-surface-2 rounded"
          >
            Cancel
          </button>
          <button
            onClick={handleSaveAnnotation}
            className="px-3 py-1.5 text-xs bg-brand-600 text-white rounded hover:bg-brand-700"
          >
            Save
          </button>
        </div>
      </div>
    );
  };

  // Sidebar annotation list
  const renderSidebar = () => {
    return (
      <div ref={sidebarRef} className="flex flex-col h-full">
        {/* Sidebar header with filters */}
        <div className="p-3 border-b bg-surface-1">
          <div className="flex items-center justify-between">
            <h3 className="font-semibold text-sm text-text-secondary">
              Annotations ({filteredAnnotations.length})
            </h3>
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={`p-1 rounded transition-colors ${showFilters ? 'bg-brand-100 text-brand-600' : 'text-text-disabled hover:text-text-secondary'}`}
            >
              <Filter className="w-4 h-4" />
            </button>
          </div>

          {showFilters && (
            <div className="mt-2 space-y-2">
              <div>
                <label className="text-xs text-text-tertiary block mb-1">Type</label>
                <select
                  value={filterType}
                  onChange={(e) => setFilterType(e.target.value)}
                  className="w-full text-xs border border-border-default rounded px-2 py-1"
                >
                  <option value="all">All types</option>
                  {Object.entries(ANNOTATION_TYPES).map(([type, config]) => (
                    <option key={type} value={type}>{config.label}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-xs text-text-tertiary block mb-1">Author</label>
                <select
                  value={filterAuthor}
                  onChange={(e) => setFilterAuthor(e.target.value)}
                  className="w-full text-xs border border-border-default rounded px-2 py-1"
                >
                  <option value="all">All authors</option>
                  {Object.entries(authors).map(([id, name]) => (
                    <option key={id} value={id}>{name}</option>
                  ))}
                </select>
              </div>
              <button
                onClick={() => setShowResolved(!showResolved)}
                className="flex items-center gap-1 text-xs text-text-tertiary hover:text-text-secondary"
              >
                {showResolved ? <Eye className="w-3 h-3" /> : <EyeOff className="w-3 h-3" />}
                {showResolved ? 'Showing resolved' : 'Hiding resolved'}
              </button>
            </div>
          )}
        </div>

        {/* Annotation list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="p-4 text-center text-text-disabled text-sm">Loading annotations...</div>
          ) : filteredAnnotations.length === 0 ? (
            <div className="p-4 text-center text-text-disabled text-sm">
              {annotations.length === 0
                ? 'No annotations yet'
                : 'No annotations match filters'}
            </div>
          ) : (
            filteredAnnotations.map(annotation => {
              const isSelected = selectedAnnotation?.id === annotation.id;
              const TypeIcon = ANNOTATION_TYPES[annotation.annotation_type]?.icon || MessageSquare;
              const isResolved = annotation.workflow_state === 'resolved';

              return (
                <div
                  key={annotation.id}
                  className={`border-b border-border-subtle transition-colors ${
                    isSelected ? 'bg-brand-50' : 'hover:bg-surface-1'
                  } ${isResolved ? 'opacity-60' : ''}`}
                >
                  {/* Annotation header */}
                  <div
                    className="p-3 cursor-pointer"
                    onClick={() => scrollToAnnotation(annotation)}
                  >
                    <div className="flex items-start gap-2">
                      <div
                        className="w-5 h-5 rounded flex items-center justify-center flex-shrink-0 mt-0.5"
                        style={{ backgroundColor: annotation.color || '#ccc' }}
                      >
                        <TypeIcon className="w-3 h-3 text-white" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between mb-0.5">
                          <span className="text-xs font-medium text-text-secondary truncate">
                            {annotation.user?.name || `User ${annotation.user_id}`}
                          </span>
                          <span className="text-xs text-text-disabled flex-shrink-0 ml-2">
                            {formatTimestamp(annotation.created_at)}
                          </span>
                        </div>
                        {annotation.content && (
                          <p className="text-sm text-text-secondary line-clamp-2">{annotation.content}</p>
                        )}
                        {!annotation.content && annotation.annotation_type === 'highlight' && (
                          <p className="text-xs text-text-disabled italic">Text highlight</p>
                        )}
                        {!annotation.content && annotation.annotation_type === 'strikethrough' && (
                          <p className="text-xs text-text-disabled italic">Strikethrough</p>
                        )}
                        {!annotation.content && annotation.annotation_type === 'freehand' && (
                          <p className="text-xs text-text-disabled italic">Freehand drawing</p>
                        )}
                        <div className="flex items-center gap-2 mt-1.5">
                          <span className="text-xs text-text-disabled">
                            p.{annotation.page_number}
                          </span>
                          {isResolved && (
                            <span className="text-xs text-accent-success flex items-center gap-0.5">
                              <Check className="w-3 h-3" /> Resolved
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Expanded view with replies and actions */}
                  {isSelected && (
                    <div className="px-3 pb-3 border-t border-border-subtle">
                      {/* Actions */}
                      {!readOnly && (
                        <div className="flex items-center gap-2 py-2">
                          <button
                            onClick={() => handleToggleResolve(annotation)}
                            className={`flex items-center gap-1 text-xs px-2 py-1 rounded transition-colors ${
                              isResolved
                                ? 'text-accent-warning hover:bg-accent-warning/10'
                                : 'text-accent-success hover:bg-accent-success/10'
                            }`}
                          >
                            {isResolved ? (
                              <>
                                <Clock className="w-3 h-3" /> Unresolve
                              </>
                            ) : (
                              <>
                                <Check className="w-3 h-3" /> Resolve
                              </>
                            )}
                          </button>
                          {(annotation.user_id === user?.id || !readOnly) && (
                            <button
                              onClick={() => handleDeleteAnnotation(annotation.id)}
                              className="flex items-center gap-1 text-xs text-accent-danger hover:bg-accent-danger/10 px-2 py-1 rounded"
                            >
                              <X className="w-3 h-3" /> Delete
                            </button>
                          )}
                        </div>
                      )}

                      {/* Replies */}
                      {annotation.replies && annotation.replies.length > 0 && (
                        <div className="space-y-2 ml-4 border-l-2 border-border-default pl-3 mb-2">
                          {annotation.replies.map(reply => (
                            <div key={reply.id} className="text-sm">
                              <div className="flex items-center justify-between">
                                <span className="text-xs font-medium text-text-secondary">
                                  {reply.user?.name || `User ${reply.user_id}`}
                                </span>
                                <span className="text-xs text-text-disabled">
                                  {formatTimestamp(reply.created_at)}
                                </span>
                              </div>
                              <p className="text-xs text-text-secondary mt-0.5">{reply.content}</p>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* Reply input */}
                      {!readOnly && (
                        <div className="flex gap-1 mt-1">
                          <input
                            type="text"
                            value={replyText}
                            onChange={(e) => setReplyText(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter' && replyText.trim()) {
                                e.stopPropagation();
                                handleReply(annotation.id);
                              }
                            }}
                            placeholder="Reply..."
                            className="flex-1 text-xs border border-border-default rounded px-2 py-1 focus:outline-none focus:ring-1 focus:ring-brand-500"
                          />
                          <button
                            onClick={() => handleReply(annotation.id)}
                            disabled={!replyText.trim()}
                            className="text-xs bg-brand-600 text-white px-2 py-1 rounded hover:bg-brand-700 disabled:bg-gray-300 disabled:cursor-not-allowed"
                          >
                            Reply
                          </button>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>
    );
  };

  return (
    <div className="flex flex-col h-full bg-surface-0 rounded-lg shadow overflow-hidden">
      {/* Toolbar */}
      {renderToolbar()}

      <div className="flex flex-1 overflow-hidden">
        {/* Document content area */}
        <div className="flex-1 overflow-auto relative">
          <div
            ref={contentRef}
            className={`p-6 min-h-full relative ${
              activeTool ? 'cursor-crosshair' : 'cursor-default'
            }`}
            onClick={handleContentClick}
          >
            {/* Document title */}
            {documentTitle && (
              <h2 className="text-lg font-semibold text-text-primary mb-4 pb-2 border-b border-border-default">
                {documentTitle}
              </h2>
            )}

            {/* Rendered document content */}
            {renderHighlightedContent()}

            {/* Annotation overlays */}
            {renderAnnotationOverlays()}

            {/* Freehand SVG overlay */}
            <svg
              ref={svgRef}
              className="absolute inset-0 w-full h-full pointer-events-none"
              style={{ pointerEvents: activeTool === 'freehand' ? 'auto' : 'none' }}
              onMouseDown={handleDrawStart}
              onMouseMove={handleDrawMove}
              onMouseUp={handleDrawEnd}
              onMouseLeave={handleDrawEnd}
            >
              {renderFreehandOverlays()}
              {isDrawing && currentPath && (
                <path
                  d={currentPath}
                  fill="none"
                  stroke={activeColor}
                  strokeWidth={2}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  opacity={0.7}
                />
              )}
            </svg>
          </div>

          {/* Pending annotation save dialog */}
          {renderPendingAnnotation()}
        </div>

        {/* Annotation sidebar */}
        <div className="w-72 border-l border-border-default flex-shrink-0 overflow-hidden flex flex-col bg-surface-0">
          {renderSidebar()}
        </div>
      </div>
    </div>
  );
};

export default DocumentViewer;
