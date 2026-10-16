/* global describe, test, expect */
import { render, screen } from '@testing-library/react';
import DesignedPageView, { PageContent, PageFrame, PageScope } from '../DesignedPageView';
import { PAGE_FRAME_CONTAINMENT } from '../../contentContainment';
import RichContentViewer from '../../RichContentViewer';
import SafeHTML from '../../SafeHTML';

const designed = (over = {}) => ({
  page_id: 42,
  url: 'home',
  title: 'Home',
  body: '<div id="pd-doc" class="paper-design" data-pd-doc="doc" data-pd-v="1"><h2>Welcome</h2></div>',
  scoped_css: '#paper-page-42 h2 { color: var(--pd-c1, #12355b); }',
  design: { version: 1, id: 'doc' },
  ...over,
});

function expectContained(el) {
  expect(el.style.contain).toBe('layout paint');
  expect(el.style.isolation).toBe('isolate');
  expect(el.style.position).toBe('relative');
}

describe('DesignedPageView', () => {
  test('writes the scoped stylesheet as text, never as HTML', () => {
    const hostile = '#paper-page-42 h2{color:red}</style><img id="pwn" src="x" onerror="alert(1)"><style>';
    const { container } = render(<DesignedPageView page={designed({ scoped_css: hostile })} />);
    const style = container.querySelector('style');
    expect(style).not.toBeNull();
    expect(style.textContent).toBe(hostile);
    expect(style.children).toHaveLength(0);
    expect(document.getElementById('pwn')).toBeNull();
    expect(container.querySelectorAll('img')).toHaveLength(0);
  });

  test('renders the body in a contained scope root with no prose wrapper', () => {
    const { container } = render(<DesignedPageView page={designed()} />);
    const root = container.querySelector('#paper-page-42');
    expect(root).not.toBeNull();
    expect(root).toHaveClass('paper-page');
    expectContained(root);
    expect(screen.getByRole('heading', { name: 'Welcome' })).toBeInTheDocument();
    expect(container.querySelector('.prose')).toBeNull();
    expect(container.querySelector('.reading-surface')).toBeNull();
    // The stylesheet is a sibling of the root, so :first-child rules in it
    // still match the page's own first element.
    expect(root.querySelector('style')).toBeNull();
    expect(root.firstElementChild).toHaveClass('paper-design');
  });

  test('keeps designed markup: data-pd-*, landmarks, picture/srcset, var() styles', () => {
    const body = [
      '<header data-pd-section="hero" style="background: var(--pd-c2, #fff); position: absolute; z-index: 3">Top</header>',
      '<nav aria-label="Course pages"><a href="/courses/1/pages/start">Start</a></nav>',
      '<picture><source media="(min-width: 800px)" srcset="/design-assets/themes/t/banner@2x.webp 2x" type="image/webp">',
      '<img src="/design-assets/themes/t/banner.webp" srcset="/design-assets/themes/t/banner@2x.webp 2x" sizes="100vw" loading="lazy" alt="Banner"></picture>',
      '<footer>Bottom</footer>',
    ].join('');
    const { container } = render(<DesignedPageView page={designed({ body })} />);
    const header = container.querySelector('header');
    expect(header).toHaveAttribute('data-pd-section', 'hero');
    expect(header.getAttribute('style')).toContain('var(--pd-c2, #fff)');
    expect(header.style.position).toBe('absolute');
    expect(screen.getByRole('navigation', { name: 'Course pages' })).toBeInTheDocument();
    const source = container.querySelector('picture source');
    expect(source).toHaveAttribute('media', '(min-width: 800px)');
    expect(source).toHaveAttribute('srcset', '/design-assets/themes/t/banner@2x.webp 2x');
    const img = screen.getByAltText('Banner');
    expect(img).toHaveAttribute('srcset', '/design-assets/themes/t/banner@2x.webp 2x');
    expect(img).toHaveAttribute('sizes', '100vw');
    expect(img).toHaveAttribute('loading', 'lazy');
    expect(container.querySelector('footer')).not.toBeNull();
  });

  test('keeps the engine root and every data-pd-* attribute it writes', () => {
    const body = [
      '<div id="pd-doc7" class="paper-design pd-layout-classic-band" data-pd-doc="doc7" data-pd-v="1" data-pd-font="serif">',
      '<section data-pd-section="welcome" data-pd-type="welcome" style="background: var(--pd-c1, #fdf6e3); color: var(--pd-c2, #1a2b3c)">',
      '<a href="/courses/1/modules" data-pd-tile="modules" data-pd-button="primary" style="display: grid; gap: 1rem">',
      '<img src="/api/v1/design/icons/solid-circle/book.svg?fg=ffffff&amp;bg=12355b" data-pd-icon="book" alt="" width="96" height="96" loading="lazy" srcset="/api/v1/design/icons/solid-circle/book.svg?fg=ffffff&amp;bg=12355b 1x">',
      '<span>Modules</span></a>',
      '<img src="/design-assets/themes/t/spot.webp" srcset="/design-assets/themes/t/spot@2x.webp 2x" data-pd-art="spot" alt="" loading="lazy">',
      '</section></div>',
    ].join('');
    const { container } = render(<DesignedPageView page={designed({ body })} />);
    const root = container.querySelector('#pd-doc7');
    expect(root).toHaveClass('paper-design', 'pd-layout-classic-band');
    expect(root).toHaveAttribute('data-pd-doc', 'doc7');
    expect(root).toHaveAttribute('data-pd-v', '1');
    expect(root).toHaveAttribute('data-pd-font', 'serif');
    const section = root.querySelector('section');
    expect(section).toHaveAttribute('data-pd-section', 'welcome');
    expect(section).toHaveAttribute('data-pd-type', 'welcome');
    expect(section.getAttribute('style')).toContain('var(--pd-c1, #fdf6e3)');
    expect(section.getAttribute('style')).toContain('var(--pd-c2, #1a2b3c)');
    const tile = root.querySelector('a');
    expect(tile).toHaveAttribute('data-pd-tile', 'modules');
    expect(tile).toHaveAttribute('data-pd-button', 'primary');
    const icon = root.querySelector('[data-pd-icon]');
    expect(icon).toHaveAttribute('data-pd-icon', 'book');
    expect(icon).toHaveAttribute('loading', 'lazy');
    expect(icon.getAttribute('srcset')).toContain('/api/v1/design/icons/solid-circle/book.svg');
    const art = root.querySelector('[data-pd-art]');
    expect(art).toHaveAttribute('data-pd-art', 'spot');
    expect(art).toHaveAttribute('srcset', '/design-assets/themes/t/spot@2x.webp 2x');
  });

  test('still sanitizes: no scripts, handlers, or fixed / sticky inline positioning', () => {
    const body = [
      '<script>window.pwned = 1</script>',
      '<div id="a" style="position: fixed; top: 0">fixed</div>',
      '<div id="b" style="position: sticky">sticky</div>',
      '<div id="c" style="--p: fixed; position: var(--p)">var</div>',
      '<img id="d" src="/x.png" alt="" onerror="alert(1)">',
    ].join('');
    const { container } = render(<DesignedPageView page={designed({ body })} />);
    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelector('#a').style.position).toBe('');
    expect(container.querySelector('#a').style.top).toBe('0px');
    expect(container.querySelector('#b').style.position).toBe('');
    expect(container.querySelector('#c').style.position).toBe('');
    expect(container.querySelector('#d')).not.toHaveAttribute('onerror');
  });

  test('no stylesheet element when the page has no CSS', () => {
    const { container } = render(<DesignedPageView page={designed({ scoped_css: '' })} />);
    expect(container.querySelector('style')).toBeNull();
  });
});

describe('PageContent / PageScope', () => {
  test('ordinary page with CSS keeps its prose rendering inside the scope root', () => {
    const page = { page_id: 5, body: '<p>Hello</p>', scoped_css: '#paper-page-5 p{color:red}', design: null };
    const { container } = render(
      <PageContent page={page} fallback={<RichContentViewer content={page.body} />} />,
    );
    const root = container.querySelector('#paper-page-5');
    expectContained(root);
    expect(root.querySelector('.reading-surface .prose')).not.toBeNull();
    expect(container.querySelector('style').textContent).toBe('#paper-page-5 p{color:red}');
  });

  test('ordinary page without CSS renders just the fallback', () => {
    const page = { page_id: 5, body: '<p>Hello</p>', scoped_css: '', design: null };
    const { container } = render(<PageContent page={page} fallback={<p>plain</p>} />);
    expect(container.querySelector('#paper-page-5')).toBeNull();
    expect(container.querySelector('style')).toBeNull();
    expect(screen.getByText('plain')).toBeInTheDocument();
  });

  test('designed page ignores the fallback and renders raw', () => {
    const { container } = render(<PageContent page={designed()} fallback={<p>fallback</p>} />);
    expect(screen.queryByText('fallback')).toBeNull();
    expect(container.querySelector('#paper-page-42 .paper-design')).not.toBeNull();
  });

  test('PageScope accepts an id field as well as page_id', () => {
    const { container } = render(<PageScope page={{ id: 9, scoped_css: 'x{}' }}><span>in</span></PageScope>);
    expect(container.querySelector('#paper-page-9')).not.toBeNull();
  });
});

// The audit's repro: page CSS that grows the root's own box and switches off
// an inner viewer's containment. Scoped the way the server scopes it.
const ESCAPE_CSS =
  '#paper-page-42{padding-right:1200px;min-height:400px} #paper-page-42 .paper-contained{contain:none!important;position:static!important}';

function expectFramed(container, id) {
  const root = container.querySelector(`#${id}`);
  const frame = root.parentElement;
  expect(frame).toHaveClass('paper-page-frame');
  expect(frame).toHaveAttribute('data-paper-page-frame');
  // The frame carries nothing a page stylesheet could name, and the page
  // root is its only child.
  expect(frame.id).toBe('');
  expect(frame.children).toHaveLength(1);
  expect(frame.getAttribute('style')).toMatch(/contain: layout paint/);
  expect(frame.getAttribute('style')).toMatch(/isolation: isolate/);
  expect(frame.getAttribute('style')).toMatch(/position: relative/);
  expect(frame.getAttribute('style')).toMatch(/overflow: clip/);
  // The stylesheet sits outside the frame.
  expect(frame.querySelector('style')).toBeNull();
  return { frame, root };
}

describe('page frame (containment the page stylesheet cannot reach)', () => {
  test('the shared frame styles', () => {
    expect(PAGE_FRAME_CONTAINMENT).toEqual({
      contain: 'layout paint',
      isolation: 'isolate',
      position: 'relative',
      overflow: 'clip',
      overflowClipMargin: '0px',
    });
    expect(Object.isFrozen(PAGE_FRAME_CONTAINMENT)).toBe(true);
  });

  test('a designed page renders frame > root, the root still contained', () => {
    const { container } = render(<DesignedPageView page={designed({ scoped_css: ESCAPE_CSS })} />);
    const { frame, root } = expectFramed(container, 'paper-page-42');
    expectContained(root);
    expectContained(frame);
  });

  test('an ordinary page with CSS renders frame > root > its prose viewer', () => {
    const page = { page_id: 42, body: '<p>Hello</p>', scoped_css: ESCAPE_CSS, design: null };
    const { container } = render(<PageContent page={page} fallback={<RichContentViewer content={page.body} />} />);
    const { root } = expectFramed(container, 'paper-page-42');
    expectContained(root);
    // The inner viewer keeps its own containment for normal pages.
    const viewer = root.querySelector('.paper-contained');
    expect(viewer).not.toBeNull();
    expectContained(viewer);
  });

  test('no rule in the escape stylesheet can match the frame', () => {
    const { container } = render(<DesignedPageView page={designed({ scoped_css: ESCAPE_CSS })} />);
    const { frame } = expectFramed(container, 'paper-page-42');
    const sheet = container.querySelector('style').textContent;
    const selectors = sheet
      .replace(/\{[^}]*\}/g, ',')
      .split(',')
      .map((x) => x.trim())
      .filter(Boolean);
    expect(selectors.length).toBeGreaterThan(0);
    for (const sel of selectors) {
      expect(frame.matches(sel)).toBe(false);
      // Every scoped selector matches the root or something inside it.
      for (const el of container.querySelectorAll(sel)) expect(frame.contains(el) && el !== frame).toBe(true);
    }
    // jsdom has no layout, so assert the structure: the frame is an ancestor
    // of the scope root (selectors are scoped to the root and its
    // descendants) and keeps its own inline containment.
    expect(frame.style.overflow).toBe('clip');
    expect(frame.style.contain).toBe('layout paint');
  });

  test('PageFrame renders only its children inside the box', () => {
    const { container } = render(<PageFrame><span>in</span></PageFrame>);
    expect(container.firstElementChild).toHaveClass('paper-page-frame');
    expect(container.firstElementChild.textContent).toBe('in');
  });
});

describe('rich content containment', () => {
  test('RichContentViewer content root is a containment box', () => {
    const { container } = render(<RichContentViewer content="<p style='position:absolute;z-index:9999'>x</p>" />);
    const root = container.querySelector('.prose');
    expectContained(root);
    expect(root).toHaveClass('paper-contained');
    // Absolute positioning and z-index survive; they stay inside the box.
    expect(root.querySelector('p').style.position).toBe('absolute');
  });

  test('SafeHTML root is a containment box and keeps caller styles', () => {
    const { container } = render(<SafeHTML html="<p>x</p>" className="prose" style={{ color: 'red', contain: 'none' }} />);
    const root = container.firstElementChild;
    expectContained(root);
    expect(root.style.color).toBe('red');
    expect(root).toHaveClass('prose', 'paper-contained');
  });
});
