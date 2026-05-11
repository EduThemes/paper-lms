import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.jsx'
import './index.css'
import './i18n'
import { AuthProvider } from './contexts/AuthContext'
import ErrorBoundary from './components/ErrorBoundary'
import { LiveRegionProvider } from './components/LiveRegion'
import { Toaster } from './components/ui/toaster'
import { TooltipProvider } from './components/ui/tooltip'
import { ThemeProvider } from './contexts/ThemeContext'
import { ReadingPrefsProvider } from './contexts/ReadingPrefsContext'
import { registerServiceWorker } from './service-worker-register'

registerServiceWorker()

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <ErrorBoundary>
      <ThemeProvider>
        <ReadingPrefsProvider>
          <AuthProvider>
            <LiveRegionProvider>
              <TooltipProvider delayDuration={200}>
                <App />
                <Toaster />
              </TooltipProvider>
            </LiveRegionProvider>
          </AuthProvider>
        </ReadingPrefsProvider>
      </ThemeProvider>
    </ErrorBoundary>
  </React.StrictMode>,
)
