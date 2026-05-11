export function registerServiceWorker() {
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker
        .register('/sw.js')
        .then((reg) => {
          console.log('SW registered:', reg.scope);

          // Check for updates periodically (every 60 minutes)
          setInterval(() => {
            reg.update();
          }, 60 * 60 * 1000);

          // Listen for new service worker activation
          reg.addEventListener('updatefound', () => {
            const newWorker = reg.installing;
            if (newWorker) {
              newWorker.addEventListener('statechange', () => {
                if (
                  newWorker.state === 'activated' &&
                  navigator.serviceWorker.controller
                ) {
                  // New version available - the user will get it on next navigation
                  console.log('New SW version available');
                }
              });
            }
          });
        })
        .catch((err) => console.log('SW registration failed:', err));
    });
  }
}
