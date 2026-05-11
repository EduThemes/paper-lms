import { useState, useEffect, useCallback, useMemo, memo } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ChevronRight, ChevronDown, Plus, Trash2, Eye, EyeOff,
  FileText, PenTool, HelpCircle, ExternalLink, Minus, Book,
  GripVertical, MessageSquare, X, Pencil, Check, Lock, Settings2,
  MoreHorizontal, Indent as IndentIncrease, Outdent as IndentDecrease, ArrowRight,
  ArrowUp, ArrowDown, Heading, Copy,
} from 'lucide-react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragOverlay,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import ModuleSettingsModal from '../components/ModuleSettingsModal';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuLabel,
} from '@/components/ui/dropdown-menu';
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

const ITEM_ICONS = {
  Page: FileText,
  Assignment: PenTool,
  Quiz: HelpCircle,
  Discussion: MessageSquare,
  ExternalUrl: ExternalLink,
  SubHeader: Minus,
};

const ITEM_TYPE_OPTIONS = [
  { value: 'SubHeader', label: 'Text Header' },
  { value: 'Assignment', label: 'Assignment' },
  { value: 'Quiz', label: 'Quiz' },
  { value: 'Page', label: 'Page' },
  { value: 'Discussion', label: 'Discussion' },
  { value: 'ExternalUrl', label: 'External URL' },
];

const MAX_INDENT = 5;
const INDENT_REM = 1.5;

const renderItemIcon = (type, className = 'w-4 h-4 text-text-tertiary flex-shrink-0') => {
  const Icon = ITEM_ICONS[type] || Book;
  return <Icon className={className} />;
};

// --- Tooltip-wrapped icon button (composition over duplication) ---
const IconTooltipButton = memo(function IconTooltipButton({ label, children, className, ...props }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button className={className} {...props}>{children}</button>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
});

// --- Drag handle (shared affordance) ---
const DragHandle = memo(function DragHandle({ size = 'md', className, ...props }) {
  const dim = size === 'lg' ? 'w-5 h-5' : 'w-4 h-4';
  return (
    <button
      className={cn(
        'flex-shrink-0 text-gray-300 hover:text-text-secondary cursor-grab active:cursor-grabbing touch-none transition-colors',
        className
      )}
      aria-label="Drag to reorder"
      {...props}
    >
      <GripVertical className={dim} />
    </button>
  );
});

// --- Sortable wrappers (drag wiring intentionally unchanged) ---
const SortableModule = ({ module, children, isTeacher, disabled }) => {
  const {
    attributes, listeners, setNodeRef, transform, transition, isDragging,
  } = useSortable({ id: `module-${module.id}`, disabled });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    position: 'relative',
    zIndex: isDragging ? 50 : 'auto',
  };
  return (
    <div ref={setNodeRef} style={style}>
      {children({ dragHandleProps: isTeacher ? { ...attributes, ...listeners } : {} })}
    </div>
  );
};

const SortableItem = ({ item, children, isTeacher, disabled }) => {
  const {
    attributes, listeners, setNodeRef, transform, transition, isDragging,
  } = useSortable({ id: `item-${item.id}`, disabled });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div ref={setNodeRef} style={style}>
      {children({ dragHandleProps: isTeacher ? { ...attributes, ...listeners } : {} })}
    </div>
  );
};

// --- Module Item Row ---
const ModuleItemRow = memo(function ModuleItemRow({
  module,
  item,
  isTeacher,
  isEditing,
  editTitle,
  setEditTitle,
  onSaveEdit,
  onCancelEdit,
  onStartEdit,
  onTogglePublish,
  onDelete,
  onIndent,
  onOutdent,
  onMoveTo,
  onMoveUp,
  onMoveDown,
  otherModules,
  canMoveUp,
  canMoveDown,
  itemLink,
  dragHandleProps,
}) {
  const indent = item.indent || 0;
  const isHeader = item.type === 'SubHeader';
  const isExternal = item.type === 'ExternalUrl';
  const padLeft = `${1 + indent * INDENT_REM}rem`;

  if (isEditing) {
    return (
      <div className="flex items-center py-2 px-4 bg-brand-50" style={{ paddingLeft: padLeft }}>
        {renderItemIcon(item.type)}
        <input
          type="text"
          value={editTitle}
          onChange={(e) => setEditTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') onSaveEdit();
            if (e.key === 'Escape') onCancelEdit();
          }}
          className="flex-1 mx-2 border border-blue-300 rounded-md px-3 py-1 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
          autoFocus
        />
        <IconTooltipButton label="Save" onClick={onSaveEdit} className="p-1 text-accent-success hover:bg-accent-success/10 rounded">
          <Check className="w-3.5 h-3.5" />
        </IconTooltipButton>
        <IconTooltipButton label="Cancel" onClick={onCancelEdit} className="p-1 text-text-secondary hover:bg-surface-2 rounded">
          <X className="w-3.5 h-3.5" />
        </IconTooltipButton>
      </div>
    );
  }

  const titleNode = (
    <div className="flex items-center gap-3 flex-1 min-w-0">
      {renderItemIcon(item.type)}
      <span className={cn(
        'text-sm flex-1 truncate',
        isHeader ? 'font-semibold text-text-secondary' : 'text-text-primary'
      )}>
        {item.title}
      </span>
      {!item.published && isTeacher && (
        <Badge variant="outline" className="text-xs font-normal">Unpublished</Badge>
      )}
    </div>
  );

  const titleWrap = itemLink ? (
    isExternal ? (
      <a
        href={itemLink}
        target={item.new_tab ? '_blank' : '_self'}
        rel={item.new_tab ? 'noopener noreferrer' : undefined}
        className="flex items-center gap-3 flex-1 min-w-0 hover:text-brand-600"
      >
        {titleNode}
      </a>
    ) : (
      <Link to={itemLink} className="flex items-center gap-3 flex-1 min-w-0 hover:text-brand-600">
        {titleNode}
      </Link>
    )
  ) : (
    <div className="flex items-center gap-3 flex-1 min-w-0">{titleNode}</div>
  );

  return (
    <div
      className="group relative flex items-center py-2 px-4 hover:bg-surface-1"
      style={{ paddingLeft: padLeft }}
    >
      {/* Indent rail (Notion-style): vertical line + horizontal tick */}
      {indent > 0 && (
        <>
          <span
            aria-hidden
            className="absolute top-0 bottom-0 w-px bg-border-default"
            style={{ left: `${1 + (indent - 1) * INDENT_REM + 0.4}rem` }}
          />
          <span
            aria-hidden
            className="absolute top-1/2 h-px bg-border-default"
            style={{
              left: `${1 + (indent - 1) * INDENT_REM + 0.4}rem`,
              width: `${INDENT_REM - 0.4}rem`,
            }}
          />
        </>
      )}

      {isTeacher && (
        <DragHandle
          size="sm"
          className="mr-2 opacity-0 group-hover:opacity-100"
          {...dragHandleProps}
        />
      )}

      {titleWrap}

      {isTeacher && !isHeader && (
        <IconTooltipButton
          label={item.published ? 'Unpublish' : 'Publish'}
          onClick={() => onTogglePublish(item)}
          className={cn(
            'p-1.5 flex-shrink-0 rounded transition-colors',
            item.published
              ? 'text-accent-success hover:bg-accent-success/10'
              : 'text-text-disabled hover:bg-surface-2'
          )}
        >
          {item.published ? <Eye className="w-3.5 h-3.5" /> : <EyeOff className="w-3.5 h-3.5" />}
        </IconTooltipButton>
      )}

      {isTeacher && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 flex-shrink-0 text-text-tertiary hover:text-text-primary"
              aria-label="Item actions"
            >
              <MoreHorizontal className="w-4 h-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-48">
            <DropdownMenuItem onClick={() => onStartEdit(item)}>
              <Pencil className="w-4 h-4" /> Edit title
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={indent >= MAX_INDENT}
              onClick={() => onIndent(item)}
            >
              <IndentIncrease className="w-4 h-4" /> Indent
              <span className="ml-auto text-xs tracking-widest opacity-60">Tab</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={indent <= 0}
              onClick={() => onOutdent(item)}
            >
              <IndentDecrease className="w-4 h-4" /> Outdent
              <span className="ml-auto text-xs tracking-widest opacity-60">⇧Tab</span>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!canMoveUp} onClick={() => onMoveUp(item)}>
              <ArrowUp className="w-4 h-4" /> Move up
            </DropdownMenuItem>
            <DropdownMenuItem disabled={!canMoveDown} onClick={() => onMoveDown(item)}>
              <ArrowDown className="w-4 h-4" /> Move down
            </DropdownMenuItem>
            {otherModules.length > 0 && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <ArrowRight className="w-4 h-4" /> Move to…
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="max-h-72 overflow-y-auto">
                  <DropdownMenuLabel className="text-xs text-text-tertiary">Module</DropdownMenuLabel>
                  {otherModules.map((m) => (
                    <DropdownMenuItem key={m.id} onClick={() => onMoveTo(item, m.id)}>
                      {m.name}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              onClick={() => onDelete(module.id, item.id)}
              className="text-accent-danger focus:text-accent-danger focus:bg-accent-danger/10"
            >
              <Trash2 className="w-4 h-4" /> Delete
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
});

// --- Module Header Row ---
const ModuleRow = memo(function ModuleRow({
  module,
  isTeacher,
  expanded,
  onToggleExpand,
  isRenaming,
  editName,
  setEditName,
  onSaveRename,
  onCancelRename,
  onStartRename,
  onTogglePublish,
  onOpenSettings,
  onAddItem,
  onAddHeader,
  onDelete,
  dragHandleProps,
}) {
  const itemCount = module.items_count || module.items?.length || 0;

  if (isRenaming) {
    return (
      <div className="flex items-center border-b">
        {isTeacher && <DragHandle size="lg" className="pl-3 pr-1 py-3" {...dragHandleProps} />}
        <div className="flex items-center gap-2 flex-1 px-4 py-2">
          <input
            type="text"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') onSaveRename();
              if (e.key === 'Escape') onCancelRename();
            }}
            className="flex-1 border border-blue-300 rounded-md px-3 py-1.5 text-sm font-semibold focus:outline-none focus:ring-2 focus:ring-brand-500"
            autoFocus
          />
          <IconTooltipButton label="Save" onClick={onSaveRename} className="p-1.5 text-accent-success hover:bg-accent-success/10 rounded">
            <Check className="w-4 h-4" />
          </IconTooltipButton>
          <IconTooltipButton label="Cancel" onClick={onCancelRename} className="p-1.5 text-text-secondary hover:bg-surface-2 rounded">
            <X className="w-4 h-4" />
          </IconTooltipButton>
        </div>
      </div>
    );
  }

  return (
    <div className="flex items-center border-b">
      {isTeacher && <DragHandle size="lg" className="pl-3 pr-1 py-3" {...dragHandleProps} />}
      <button
        className="flex items-center gap-3 flex-1 px-4 py-3 text-left hover:bg-surface-1"
        onClick={onToggleExpand}
        aria-expanded={!!expanded}
      >
        {expanded
          ? <ChevronDown className="w-5 h-5 text-text-disabled flex-shrink-0" />
          : <ChevronRight className="w-5 h-5 text-text-disabled flex-shrink-0" />}
        <span className="font-semibold text-text-primary">{module.name}</span>
        <span className="text-xs text-text-secondary ml-2">
          {itemCount} {itemCount === 1 ? 'item' : 'items'}
        </span>
        {!module.published && (
          <Badge variant="outline" className="text-xs font-normal ml-2">Unpublished</Badge>
        )}
      </button>

      {isTeacher && (
        <div className="flex items-center gap-1 px-3">
          <IconTooltipButton
            label={module.published ? 'Unpublish module' : 'Publish module'}
            onClick={onTogglePublish}
            className={cn(
              'p-1.5 rounded transition-colors',
              module.published
                ? 'text-accent-success hover:bg-accent-success/10'
                : 'text-text-disabled hover:bg-surface-2'
            )}
            aria-label={module.published ? 'Unpublish module' : 'Publish module'}
          >
            {module.published ? <Eye className="w-4 h-4" /> : <EyeOff className="w-4 h-4" />}
          </IconTooltipButton>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-text-tertiary hover:text-text-primary"
                aria-label="Module actions"
              >
                <MoreHorizontal className="w-4 h-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem onClick={onStartRename}>
                <Pencil className="w-4 h-4" /> Edit name
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onOpenSettings}>
                <Settings2 className="w-4 h-4" /> Module settings…
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={onAddItem}>
                <Plus className="w-4 h-4" /> Add item
              </DropdownMenuItem>
              <DropdownMenuItem onClick={onAddHeader}>
                <Heading className="w-4 h-4" /> Add text header
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {/* TODO: wire when backend supports module duplicate/move APIs */}
              <DropdownMenuItem disabled>
                <Copy className="w-4 h-4" /> Duplicate
              </DropdownMenuItem>
              <DropdownMenuItem disabled>
                <ArrowRight className="w-4 h-4" /> Move to…
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                onClick={onDelete}
                className="text-accent-danger focus:text-accent-danger focus:bg-accent-danger/10"
              >
                <Trash2 className="w-4 h-4" /> Delete module
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
    </div>
  );
});

const ModulesPage = () => {
  const { courseId } = useParams();
  // useAuth retained for parity; user not currently consumed but the hook subscribes the page to auth changes.
  useAuth();
  const [modules, setModules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [expandedModules, setExpandedModules] = useState({});
  const [showCreateModule, setShowCreateModule] = useState(false);
  const [newModuleName, setNewModuleName] = useState('');
  const [creating, setCreating] = useState(false);
  const [addingItemTo, setAddingItemTo] = useState(null);
  const [newItem, setNewItem] = useState({ title: '', type: 'SubHeader', external_url: '', new_tab: false });
  const [editingModuleId, setEditingModuleId] = useState(null);
  const [editModuleName, setEditModuleName] = useState('');
  const [editingItemId, setEditingItemId] = useState(null);
  const [editItemTitle, setEditItemTitle] = useState('');
  const [activeId, setActiveId] = useState(null);
  const [dragType, setDragType] = useState(null);
  const [prerequisites, setPrerequisites] = useState({});
  const [settingsModuleId, setSettingsModuleId] = useState(null);

  const isTeacher = useIsTeacher(courseId);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  const fetchModules = useCallback(async () => {
    try {
      const result = await api.getModules(courseId, 1, 100, true);
      const mods = result.data || [];
      setModules(mods);
      setExpandedModules(prev => {
        const merged = { ...prev };
        mods.forEach(m => { if (merged[m.id] === undefined) merged[m.id] = true; });
        return merged;
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => { fetchModules(); }, [fetchModules]);

  // Stable signature so the prereq fetcher doesn't churn on every items mutation.
  const moduleIdsKey = useMemo(() => modules.map(m => m.id).join(','), [modules]);

  useEffect(() => {
    if (!moduleIdsKey) return;
    const ids = moduleIdsKey.split(',').filter(Boolean).map(Number);
    let cancelled = false;
    (async () => {
      const prereqMap = {};
      await Promise.all(ids.map(async (id) => {
        try {
          const result = await api.getModulePrerequisites(courseId, id);
          prereqMap[id] = result?.prerequisite_module_ids || [];
        } catch {
          prereqMap[id] = [];
        }
      }));
      if (!cancelled) setPrerequisites(prereqMap);
    })();
    return () => { cancelled = true; };
  }, [moduleIdsKey, courseId]);

  const getModuleName = useCallback((moduleId) => {
    const mod = modules.find(m => m.id === moduleId);
    return mod ? mod.name : `Module ${moduleId}`;
  }, [modules]);

  const toggleModule = useCallback((moduleId) => {
    setExpandedModules(prev => ({ ...prev, [moduleId]: !prev[moduleId] }));
  }, []);

  const handleCreateModule = async (e) => {
    e.preventDefault();
    if (!newModuleName.trim()) return;
    setCreating(true);
    try {
      await api.createModule(courseId, { name: newModuleName.trim(), position: modules.length + 1 });
      setNewModuleName('');
      setShowCreateModule(false);
      await fetchModules();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteModule = useCallback(async (moduleId) => {
    if (!window.confirm('Delete this module and all its items?')) return;
    try {
      await api.deleteModule(courseId, moduleId);
      await fetchModules();
    } catch (err) {
      setError(err.message);
    }
  }, [courseId, fetchModules]);

  const handleTogglePublishModule = useCallback(async (module) => {
    const newPublished = !module.published;
    try {
      await api.updateModule(courseId, module.id, { published: newPublished });
      setModules(prev => prev.map(m =>
        m.id === module.id
          ? { ...m, published: newPublished, workflow_state: newPublished ? 'active' : 'unpublished' }
          : m
      ));
    } catch (err) {
      setError(err.message);
    }
  }, [courseId]);

  const startRenameModule = useCallback((module) => {
    setEditingModuleId(module.id);
    setEditModuleName(module.name);
  }, []);

  const handleRenameModule = useCallback(async (moduleId) => {
    if (!editModuleName.trim()) return;
    const trimmed = editModuleName.trim();
    try {
      await api.updateModule(courseId, moduleId, { name: trimmed });
      setModules(prev => prev.map(m => (m.id === moduleId ? { ...m, name: trimmed } : m)));
    } catch (err) {
      setError(err.message);
    } finally {
      setEditingModuleId(null);
      setEditModuleName('');
    }
  }, [courseId, editModuleName]);

  const openAddItem = useCallback((moduleId, type = 'Assignment') => {
    setAddingItemTo(moduleId);
    setNewItem({ title: '', type, external_url: '', new_tab: false });
  }, []);

  const handleAddItem = async (e, moduleId) => {
    e.preventDefault();
    if (!newItem.title.trim()) return;
    setCreating(true);
    try {
      const itemPayload = {
        title: newItem.title.trim(),
        type: newItem.type,
        position: (modules.find(m => m.id === moduleId)?.items?.length || 0) + 1,
      };
      if (newItem.type === 'ExternalUrl') {
        itemPayload.external_url = newItem.external_url;
        itemPayload.new_tab = newItem.new_tab;
      }
      await api.createModuleItem(courseId, moduleId, itemPayload);
      setAddingItemTo(null);
      setNewItem({ title: '', type: 'SubHeader', external_url: '', new_tab: false });
      await fetchModules();
    } catch (err) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleDeleteItem = useCallback(async (moduleId, itemId) => {
    try {
      await api.deleteModuleItem(courseId, moduleId, itemId);
      await fetchModules();
    } catch (err) {
      setError(err.message);
    }
  }, [courseId, fetchModules]);

  const handleToggleItemPublish = useCallback(async (moduleId, item) => {
    const newPublished = !item.published;
    setModules(prev => prev.map(m =>
      m.id === moduleId
        ? {
            ...m,
            items: (m.items || []).map(i =>
              i.id === item.id
                ? { ...i, published: newPublished, workflow_state: newPublished ? 'active' : 'unpublished' }
                : i
            ),
          }
        : m
    ));
    try {
      await api.updateModuleItem(courseId, moduleId, item.id, { published: newPublished });
    } catch (err) {
      setError(err.message);
      await fetchModules();
    }
  }, [courseId, fetchModules]);

  const startRenameItem = useCallback((item) => {
    setEditingItemId(item.id);
    setEditItemTitle(item.title);
  }, []);

  const cancelRenameItem = useCallback(() => {
    setEditingItemId(null);
    setEditItemTitle('');
  }, []);

  const handleRenameItem = useCallback(async (moduleId, itemId) => {
    if (!editItemTitle.trim()) return;
    const newTitle = editItemTitle.trim();
    const prevModules = modules;
    setModules(prev => prev.map(m =>
      m.id === moduleId
        ? { ...m, items: (m.items || []).map(i => (i.id === itemId ? { ...i, title: newTitle } : i)) }
        : m
    ));
    setEditingItemId(null);
    setEditItemTitle('');
    try {
      await api.updateModuleItem(courseId, moduleId, itemId, { title: newTitle });
    } catch (err) {
      setError(err.message);
      setModules(prevModules);
    }
  }, [courseId, editItemTitle, modules]);

  const updateItemIndent = useCallback(async (moduleId, item, delta) => {
    const next = Math.max(0, Math.min(MAX_INDENT, (item.indent || 0) + delta));
    if (next === (item.indent || 0)) return;
    setModules(prev => prev.map(m =>
      m.id === moduleId
        ? { ...m, items: (m.items || []).map(i => (i.id === item.id ? { ...i, indent: next } : i)) }
        : m
    ));
    try {
      await api.updateModuleItem(courseId, moduleId, item.id, { indent: next });
    } catch (err) {
      setError(err.message);
      await fetchModules();
    }
  }, [courseId, fetchModules]);

  const handleMoveItemTo = useCallback(async (sourceModuleId, item, targetModuleId) => {
    if (sourceModuleId === targetModuleId) return;
    const target = modules.find(m => m.id === targetModuleId);
    const newPosition = (target?.items?.length || 0) + 1;
    try {
      await api.moveModuleItem(courseId, sourceModuleId, item.id, targetModuleId, newPosition);
      await fetchModules();
    } catch (err) {
      setError(err.message);
    }
  }, [courseId, fetchModules, modules]);

  const handleMoveItemWithin = useCallback(async (moduleId, item, dir) => {
    const mod = modules.find(m => m.id === moduleId);
    if (!mod) return;
    const items = mod.items || [];
    const idx = items.findIndex(i => i.id === item.id);
    const newIdx = idx + dir;
    if (idx === -1 || newIdx < 0 || newIdx >= items.length) return;
    const newItems = arrayMove(items, idx, newIdx);
    setModules(prev => prev.map(m => (m.id === moduleId ? { ...m, items: newItems } : m)));
    try {
      await api.reorderModuleItems(courseId, moduleId, newItems.map(i => i.id));
    } catch (err) {
      setError(err.message);
      await fetchModules();
    }
  }, [courseId, fetchModules, modules]);

  // --- Drag-and-Drop Handlers (UNCHANGED wiring) ---
  const handleDragStart = (event) => {
    const { active } = event;
    setActiveId(active.id);
    setDragType(String(active.id).startsWith('module-') ? 'module' : 'item');
  };

  const handleDragEnd = async (event) => {
    const { active, over } = event;
    setActiveId(null);
    setDragType(null);
    if (!over || active.id === over.id) return;

    const activeIdStr = String(active.id);
    const overIdStr = String(over.id);

    if (activeIdStr.startsWith('module-') && overIdStr.startsWith('module-')) {
      const activeModuleId = parseInt(activeIdStr.replace('module-', ''));
      const overModuleId = parseInt(overIdStr.replace('module-', ''));
      const oldIndex = modules.findIndex(m => m.id === activeModuleId);
      const newIndex = modules.findIndex(m => m.id === overModuleId);
      if (oldIndex !== -1 && newIndex !== -1) {
        const newModules = arrayMove(modules, oldIndex, newIndex);
        setModules(newModules);
        try {
          await api.reorderModules(courseId, newModules.map(m => m.id));
        } catch (err) {
          setError(err.message);
          await fetchModules();
        }
      }
      return;
    }

    if (activeIdStr.startsWith('item-') && overIdStr.startsWith('item-')) {
      const activeItemId = parseInt(activeIdStr.replace('item-', ''));
      const overItemId = parseInt(overIdStr.replace('item-', ''));
      let sourceModule = null;
      let targetModule = null;
      for (const mod of modules) {
        if ((mod.items || []).find(i => i.id === activeItemId)) sourceModule = mod;
        if ((mod.items || []).find(i => i.id === overItemId)) targetModule = mod;
      }
      if (!sourceModule || !targetModule) return;

      if (sourceModule.id === targetModule.id) {
        const items = [...(sourceModule.items || [])];
        const oldIndex = items.findIndex(i => i.id === activeItemId);
        const newIndex = items.findIndex(i => i.id === overItemId);
        if (oldIndex !== -1 && newIndex !== -1) {
          const newItems = arrayMove(items, oldIndex, newIndex);
          setModules(prev => prev.map(m => (m.id === sourceModule.id ? { ...m, items: newItems } : m)));
          try {
            await api.reorderModuleItems(courseId, sourceModule.id, newItems.map(i => i.id));
          } catch (err) {
            setError(err.message);
            await fetchModules();
          }
        }
      } else {
        const activeItem = (sourceModule.items || []).find(i => i.id === activeItemId);
        if (!activeItem) return;
        const targetItems = [...(targetModule.items || [])];
        const overIndex = targetItems.findIndex(i => i.id === overItemId);
        const newPosition = overIndex + 1;
        setModules(prev => prev.map(m => {
          if (m.id === sourceModule.id) return { ...m, items: (m.items || []).filter(i => i.id !== activeItemId) };
          if (m.id === targetModule.id) {
            const items = [...(m.items || [])];
            items.splice(overIndex, 0, { ...activeItem, module_id: targetModule.id });
            return { ...m, items };
          }
          return m;
        }));
        try {
          await api.moveModuleItem(courseId, sourceModule.id, activeItemId, targetModule.id, newPosition);
          await fetchModules();
        } catch (err) {
          setError(err.message);
          await fetchModules();
        }
      }
    }
  };

  const getItemLink = useCallback((item) => {
    if (item.type === 'Assignment' && item.content_id) return `/courses/${courseId}/assignments/${item.content_id}`;
    if (item.type === 'Quiz' && item.content_id) return `/courses/${courseId}/quizzes/${item.content_id}/take`;
    if (item.type === 'Page') {
      if (item.page_url) return `/courses/${courseId}/pages/${item.page_url}`;
      if (item.content_id) return `/courses/${courseId}/pages/${item.content_id}`;
    }
    if (item.type === 'Discussion' && item.content_id) return `/courses/${courseId}/discussions/${item.content_id}`;
    if (item.type === 'ExternalUrl' && item.url) return item.url;
    return null;
  }, [courseId]);

  const activeDragItem = useMemo(() => {
    if (!activeId) return null;
    const idStr = String(activeId);
    if (idStr.startsWith('module-')) {
      const moduleId = parseInt(idStr.replace('module-', ''));
      return modules.find(m => m.id === moduleId);
    }
    const itemId = parseInt(idStr.replace('item-', ''));
    for (const mod of modules) {
      const item = (mod.items || []).find(i => i.id === itemId);
      if (item) return item;
    }
    return null;
  }, [activeId, modules]);

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-4 py-4">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-24 w-full rounded-lg" />
          <Skeleton className="h-24 w-full rounded-lg" />
          <Skeleton className="h-24 w-full rounded-lg" />
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          &larr; Back to Course
        </Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold text-text-primary">Modules</h2>
          {isTeacher && (
            <Button onClick={() => setShowCreateModule(v => !v)} size="sm">
              {showCreateModule ? <X className="w-4 h-4" /> : <Plus className="w-4 h-4" />}
              {showCreateModule ? 'Cancel' : 'Module'}
            </Button>
          )}
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger rounded-md p-3 mb-4 text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 text-accent-danger hover:text-accent-danger font-bold">&times;</button>
        </div>
      )}

      {showCreateModule && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <form onSubmit={handleCreateModule} className="flex items-center gap-3">
            <input
              type="text"
              value={newModuleName}
              onChange={(e) => setNewModuleName(e.target.value)}
              placeholder="Module name..."
              className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              autoFocus
            />
            <Button type="submit" size="sm" disabled={creating || !newModuleName.trim()}>
              {creating ? 'Creating...' : 'Add Module'}
            </Button>
          </form>
        </div>
      )}

      {modules.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Book className="w-12 h-12 text-gray-300 mx-auto mb-3" />
          <p className="text-text-tertiary text-lg mb-1">No modules yet</p>
          {isTeacher && (
            <p className="text-text-secondary text-sm">
              Click &quot;+ Module&quot; above to create your first module.
            </p>
          )}
        </div>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragStart={handleDragStart}
          onDragEnd={handleDragEnd}
        >
          <SortableContext
            items={modules.map(m => `module-${m.id}`)}
            strategy={verticalListSortingStrategy}
          >
            <div className="space-y-4">
              {modules.map((module) => {
                const items = module.items || [];
                const otherModules = modules.filter(m => m.id !== module.id);
                const isExpanded = !!expandedModules[module.id];
                const showPrereqRow = isExpanded
                  && (prerequisites[module.id]?.length > 0 || module.require_sequential_progress);

                return (
                  <SortableModule
                    key={module.id}
                    module={module}
                    isTeacher={isTeacher}
                    disabled={!isTeacher || dragType === 'item'}
                  >
                    {({ dragHandleProps }) => (
                      <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
                        <ModuleRow
                          module={module}
                          isTeacher={isTeacher}
                          expanded={isExpanded}
                          onToggleExpand={() => toggleModule(module.id)}
                          isRenaming={editingModuleId === module.id}
                          editName={editModuleName}
                          setEditName={setEditModuleName}
                          onSaveRename={() => handleRenameModule(module.id)}
                          onCancelRename={() => { setEditingModuleId(null); setEditModuleName(''); }}
                          onStartRename={() => startRenameModule(module)}
                          onTogglePublish={() => handleTogglePublishModule(module)}
                          onOpenSettings={() => setSettingsModuleId(module.id)}
                          onAddItem={() => openAddItem(module.id, 'Assignment')}
                          onAddHeader={() => openAddItem(module.id, 'SubHeader')}
                          onDelete={() => handleDeleteModule(module.id)}
                          dragHandleProps={dragHandleProps}
                        />

                        {showPrereqRow && (
                          <div className="border-b bg-surface-1 px-4 py-2 flex items-center gap-3 flex-wrap">
                            {prerequisites[module.id]?.length > 0 && (
                              <div className="flex items-center gap-2 text-xs text-text-tertiary">
                                <Lock className="w-3.5 h-3.5 flex-shrink-0" />
                                <span>
                                  Requires: {prerequisites[module.id].map(id => getModuleName(id)).join(', ')}
                                </span>
                              </div>
                            )}
                            {module.require_sequential_progress && (
                              <Badge variant="secondary" className="text-xs font-medium bg-accent-warning/10 text-accent-warning hover:bg-accent-warning/10">
                                Sequential
                              </Badge>
                            )}
                          </div>
                        )}

                        {isExpanded && (
                          <div>
                            {items.length === 0 ? (
                              <div className="m-4 rounded-md border border-dashed border-border-strong px-4 py-6 text-center">
                                <p className="text-sm text-text-tertiary mb-2">
                                  Drop items here, or click below to add
                                </p>
                                {isTeacher && (
                                  <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => openAddItem(module.id, 'Assignment')}
                                  >
                                    <Plus className="w-4 h-4" /> Add item
                                  </Button>
                                )}
                              </div>
                            ) : (
                              <SortableContext
                                items={items.map(i => `item-${i.id}`)}
                                strategy={verticalListSortingStrategy}
                              >
                                <div className="divide-y divide-gray-100">
                                  {items.map((item, idx) => (
                                    <SortableItem
                                      key={item.id}
                                      item={item}
                                      isTeacher={isTeacher}
                                      disabled={!isTeacher || dragType === 'module'}
                                    >
                                      {({ dragHandleProps: itemDragProps }) => (
                                        <ModuleItemRow
                                          module={module}
                                          item={item}
                                          isTeacher={isTeacher}
                                          isEditing={editingItemId === item.id}
                                          editTitle={editItemTitle}
                                          setEditTitle={setEditItemTitle}
                                          onSaveEdit={() => handleRenameItem(module.id, item.id)}
                                          onCancelEdit={cancelRenameItem}
                                          onStartEdit={startRenameItem}
                                          onTogglePublish={(it) => handleToggleItemPublish(module.id, it)}
                                          onDelete={handleDeleteItem}
                                          onIndent={(it) => updateItemIndent(module.id, it, +1)}
                                          onOutdent={(it) => updateItemIndent(module.id, it, -1)}
                                          onMoveTo={(it, targetId) => handleMoveItemTo(module.id, it, targetId)}
                                          onMoveUp={(it) => handleMoveItemWithin(module.id, it, -1)}
                                          onMoveDown={(it) => handleMoveItemWithin(module.id, it, +1)}
                                          otherModules={otherModules}
                                          canMoveUp={idx > 0}
                                          canMoveDown={idx < items.length - 1}
                                          itemLink={getItemLink(item)}
                                          dragHandleProps={itemDragProps}
                                        />
                                      )}
                                    </SortableItem>
                                  ))}
                                </div>
                              </SortableContext>
                            )}

                            {addingItemTo === module.id && (
                              <div className="border-t bg-surface-1 p-4">
                                <form onSubmit={(e) => handleAddItem(e, module.id)} className="space-y-3">
                                  <div className="flex items-center gap-3">
                                    <select
                                      value={newItem.type}
                                      onChange={(e) => setNewItem({ ...newItem, type: e.target.value })}
                                      className="border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                    >
                                      {ITEM_TYPE_OPTIONS.map(opt => (
                                        <option key={opt.value} value={opt.value}>{opt.label}</option>
                                      ))}
                                    </select>
                                    <input
                                      type="text"
                                      value={newItem.title}
                                      onChange={(e) => setNewItem({ ...newItem, title: e.target.value })}
                                      placeholder="Item title..."
                                      className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                      autoFocus
                                    />
                                  </div>
                                  {newItem.type === 'ExternalUrl' && (
                                    <div className="flex items-center gap-3">
                                      <input
                                        type="url"
                                        value={newItem.external_url}
                                        onChange={(e) => setNewItem({ ...newItem, external_url: e.target.value })}
                                        placeholder="https://..."
                                        className="flex-1 border border-border-strong rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                                      />
                                      <label className="flex items-center gap-2 text-sm text-text-secondary whitespace-nowrap">
                                        <input
                                          type="checkbox"
                                          checked={newItem.new_tab}
                                          onChange={(e) => setNewItem({ ...newItem, new_tab: e.target.checked })}
                                          className="rounded border-border-strong"
                                        />
                                        New tab
                                      </label>
                                    </div>
                                  )}
                                  <div className="flex items-center gap-2 justify-end">
                                    <Button
                                      type="button"
                                      variant="ghost"
                                      size="sm"
                                      onClick={() => setAddingItemTo(null)}
                                    >
                                      Cancel
                                    </Button>
                                    <Button
                                      type="submit"
                                      size="sm"
                                      disabled={creating || !newItem.title.trim()}
                                    >
                                      {creating ? 'Adding...' : 'Add Item'}
                                    </Button>
                                  </div>
                                </form>
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    )}
                  </SortableModule>
                );
              })}
            </div>
          </SortableContext>

          <DragOverlay>
            {activeId && activeDragItem && dragType === 'module' ? (
              <div className="bg-surface-0 rounded-lg shadow-lg border-2 border-blue-400 overflow-hidden opacity-90">
                <div className="flex items-center gap-3 px-4 py-3">
                  <GripVertical className="w-5 h-5 text-blue-400" />
                  <span className="font-semibold text-text-primary">{activeDragItem.name}</span>
                  <span className="text-xs text-text-secondary ml-2">
                    {activeDragItem.items_count || activeDragItem.items?.length || 0} items
                  </span>
                </div>
              </div>
            ) : activeId && activeDragItem && dragType === 'item' ? (
              <div className="bg-surface-0 shadow-lg border-2 border-blue-400 rounded px-4 py-2 flex items-center gap-3 opacity-90">
                <GripVertical className="w-4 h-4 text-blue-400" />
                {renderItemIcon(activeDragItem.type)}
                <span className="text-sm text-text-primary">{activeDragItem.title}</span>
              </div>
            ) : null}
          </DragOverlay>
        </DndContext>
      )}

      {settingsModuleId && (() => {
        const mod = modules.find(m => m.id === settingsModuleId);
        if (!mod) return null;
        return (
          <ModuleSettingsModal
            courseId={courseId}
            module={mod}
            modules={modules}
            prerequisites={prerequisites[mod.id] || []}
            onClose={() => setSettingsModuleId(null)}
            onSave={({ prereqIds, requireSequential, itemRequirements }) => {
              setPrerequisites(prev => ({ ...prev, [mod.id]: prereqIds }));
              setModules(prev => prev.map(m =>
                m.id === mod.id
                  ? {
                      ...m,
                      require_sequential_progress: requireSequential,
                      items: (m.items || []).map(item => {
                        const req = itemRequirements[item.id];
                        if (!req) return item;
                        return {
                          ...item,
                          completion_type: req.completion_type,
                          min_score: req.completion_type === 'min_score' && req.min_score !== ''
                            ? parseFloat(req.min_score)
                            : null,
                        };
                      }),
                    }
                  : m
              ));
              setSettingsModuleId(null);
            }}
          />
        );
      })()}
    </Layout>
  );
};

export default ModulesPage;
