// Paper LMS Service Worker
// BUILD_HASH is replaced at build time by the sw-build-hash Vite plugin.
// In dev mode it stays as the literal placeholder and falls back to 'dev'.
const BUILD_HASH = '__BUILD_HASH__';
const CACHE_NAME = BUILD_HASH.startsWith('__') ? 'paper-lms-dev' : `paper-lms-${BUILD_HASH}`;
const STATIC_CACHE_LIMIT = 150;

// App shell files to pre-cache on install
const APP_SHELL = [
  '/',
  '/index.html',
  '/offline.html',
  '/manifest.json',
  '/brand/paper-logo.svg',
];

// Endpoints that should never be cached
const NO_CACHE_PATTERNS = [
  '/api/v1/login',
  '/api/v1/logout',
  '/api/v1/oauth2/',
  '/api/v1/jwks',
];

// ----- Install: pre-cache app shell -----
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      console.log('[SW] Pre-caching app shell, cache:', CACHE_NAME);
      return cache.addAll(APP_SHELL);
    })
  );
  // Activate immediately without waiting for existing clients to close
  self.skipWaiting();
});

// ----- Activate: clean old caches + trim -----
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((cacheNames) => {
      return Promise.all(
        cacheNames
          .filter((name) => name.startsWith('paper-lms-') && name !== CACHE_NAME)
          .map((name) => {
            console.log('[SW] Removing old cache:', name);
            return caches.delete(name);
          })
      );
    }).then(() => trimCache(CACHE_NAME, STATIC_CACHE_LIMIT))
  );
  // Take control of all clients immediately
  self.clients.claim();
});

// ----- Fetch: routing strategies -----
self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);

  // Only handle same-origin requests
  if (url.origin !== location.origin) {
    return;
  }

  // Skip caching for auth-related API endpoints
  if (NO_CACHE_PATTERNS.some((pattern) => url.pathname.startsWith(pattern))) {
    return;
  }

  // API calls: network-first strategy
  if (url.pathname.startsWith('/api/')) {
    event.respondWith(networkFirst(request));
    return;
  }

  // Navigation requests: network-first with offline fallback
  if (request.mode === 'navigate') {
    event.respondWith(navigationHandler(request));
    return;
  }

  // Vite content-hashed assets (e.g., /assets/index-Dmlh4wMf.js):
  // cache-first is safe — URLs change per build, so cached = correct version
  if (isHashedAsset(url.pathname)) {
    event.respondWith(cacheFirst(request));
    return;
  }

  // Non-hashed static assets (images, fonts, icons):
  // stale-while-revalidate — serve cached immediately, fetch fresh in background
  event.respondWith(staleWhileRevalidate(request));
});

// ----- Message handler: allow app to trigger SW update -----
self.addEventListener('message', (event) => {
  if (event.data === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

// ----- Detect Vite content-hashed assets -----
// Matches patterns like /assets/index-Dmlh4wMf.js or /assets/vendor-react-BkL9xqPZ.css
function isHashedAsset(pathname) {
  return /\/assets\/[^/]+-[a-zA-Z0-9]{8,}\.\w+$/.test(pathname);
}

// ----- Strategy: Network first (for API calls) -----
async function networkFirst(request) {
  try {
    const networkResponse = await fetch(request);
    // Cache successful GET responses
    if (request.method === 'GET' && networkResponse.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, networkResponse.clone());
    }
    return networkResponse;
  } catch (error) {
    const cachedResponse = await caches.match(request);
    if (cachedResponse) {
      return cachedResponse;
    }
    // Return a JSON error for API calls when offline
    return new Response(
      JSON.stringify({ errors: [{ message: 'You are offline' }] }),
      {
        status: 503,
        headers: { 'Content-Type': 'application/json' },
      }
    );
  }
}

// ----- Strategy: Cache first (for Vite hashed assets) -----
async function cacheFirst(request) {
  const cachedResponse = await caches.match(request);
  if (cachedResponse) {
    return cachedResponse;
  }
  try {
    const networkResponse = await fetch(request);
    if (request.method === 'GET' && networkResponse.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, networkResponse.clone());
    }
    return networkResponse;
  } catch (error) {
    return new Response('', { status: 504, statusText: 'Offline' });
  }
}

// ----- Strategy: Stale-while-revalidate (for non-hashed static assets) -----
async function staleWhileRevalidate(request) {
  const cache = await caches.open(CACHE_NAME);
  const cachedResponse = await cache.match(request);

  // Always fetch fresh version in background
  const fetchPromise = fetch(request)
    .then((networkResponse) => {
      if (request.method === 'GET' && networkResponse.ok) {
        cache.put(request, networkResponse.clone());
      }
      return networkResponse;
    })
    .catch(() => null);

  // Return cached immediately if available; otherwise wait for network
  if (cachedResponse) {
    return cachedResponse;
  }

  const networkResponse = await fetchPromise;
  if (networkResponse) {
    return networkResponse;
  }

  return new Response('', { status: 504, statusText: 'Offline' });
}

// ----- Strategy: Navigation handler with offline fallback -----
async function navigationHandler(request) {
  try {
    const networkResponse = await fetch(request);
    // Cache the latest version of the page
    if (networkResponse.ok) {
      const cache = await caches.open(CACHE_NAME);
      cache.put(request, networkResponse.clone());
    }
    return networkResponse;
  } catch (error) {
    // Try to return the cached index.html for SPA routing
    const cachedResponse = await caches.match('/index.html');
    if (cachedResponse) {
      return cachedResponse;
    }
    // Last resort: return the offline page
    const offlinePage = await caches.match('/offline.html');
    if (offlinePage) {
      return offlinePage;
    }
    return new Response('Offline', { status: 503 });
  }
}

// ----- Cache management: trim to max entries -----
async function trimCache(cacheName, maxEntries) {
  const cache = await caches.open(cacheName);
  const keys = await cache.keys();
  if (keys.length <= maxEntries) return;
  // Evict oldest entries (first added = first in keys array)
  const toDelete = keys.slice(0, keys.length - maxEntries);
  await Promise.all(toDelete.map((key) => cache.delete(key)));
  console.log(`[SW] Trimmed ${toDelete.length} old cache entries`);
}
