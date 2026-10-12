import React, { useEffect, useState } from 'react';
import { Puzzle } from 'lucide-react';
import { Button } from '../ui/button';
import LtiDeepLinkModal from './LtiDeepLinkModal';
import { ltiApi, ltiLaunchUrl } from '../../services/api_lti';
import { contentItemsToEditorContent } from './ltiEditorContent';

// editor_button tools per course, shared by every editor on the page.
const toolCache = new Map();

function loadEditorTools(courseId) {
  const key = String(courseId);
  if (!toolCache.has(key)) {
    toolCache.set(
      key,
      ltiApi.courseTools(courseId, 'editor_button').catch(() => {
        toolCache.delete(key);
        return [];
      })
    );
  }
  return toolCache.get(key);
}

/**
 * "Apps" toolbar button of the rich content editor (G008): lists the
 * course's editor_button LTI tools and inserts the content a tool returns
 * through deep linking. Renders nothing when the course has no such tool.
 */
export default function LtiEditorAppsButton({ editor, courseId }) {
  const [tools, setTools] = useState([]);
  const [menuOpen, setMenuOpen] = useState(false);
  const [active, setActive] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    let live = true;
    if (courseId == null) return undefined;
    loadEditorTools(courseId).then((list) => {
      if (live) setTools(Array.isArray(list) ? list : []);
    });
    return () => {
      live = false;
    };
  }, [courseId]);

  if (!editor || courseId == null || tools.length === 0) return null;

  const onResult = ({ items, error: err }) => {
    setActive(null);
    if (err) {
      setError(err);
      return;
    }
    const content = contentItemsToEditorContent(items);
    if (content.length) {
      const chain = editor.chain().focus();
      content.forEach((c) => chain.insertContent(c));
      chain.run();
    }
  };

  return (
    <div className="relative">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="h-8 w-8"
        aria-label="Apps"
        title="Apps"
        aria-haspopup="menu"
        aria-expanded={menuOpen}
        onClick={() => setMenuOpen((o) => !o)}
      >
        <Puzzle />
      </Button>
      {menuOpen && (
        <div role="menu" className="absolute left-0 top-full z-50 mt-1 flex min-w-48 flex-col rounded-md border bg-popover p-1 shadow-md">
          {tools.map((t) => (
            <button
              key={t.id}
              type="button"
              role="menuitem"
              className="flex items-center gap-2 rounded px-3 py-1 text-left text-sm hover:bg-accent"
              onClick={() => {
                setMenuOpen(false);
                setError(null);
                setActive(t);
              }}
            >
              {t.icon_url ? <img src={t.icon_url} alt="" className="h-4 w-4" /> : <Puzzle className="h-4 w-4" />}
              {t.text || t.name}
            </button>
          ))}
        </div>
      )}
      {error && (
        <span role="alert" className="absolute left-0 top-full mt-1 whitespace-nowrap text-xs text-accent-danger">
          {error}
        </span>
      )}
      <LtiDeepLinkModal
        open={Boolean(active)}
        title={active ? active.text || active.name : ''}
        src={active ? ltiLaunchUrl(courseId, active.id, { placement: 'editor_button', deep_link: 'editor' }) : ''}
        width={active?.selection_width}
        height={active?.selection_height}
        onResult={onResult}
        onClose={() => setActive(null)}
      />
    </div>
  );
}
