import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import App from './App.tsx'
import { LiveAlertsProvider } from './context/liveAlerts.tsx'
import { MapModeProvider } from './context/mapMode.tsx'
import { ToastProvider } from './context/toast.tsx'
// Third-party CSS is imported here rather than through Tailwind's @import: geoman's inline SVGs
// break the CSS minifier when Tailwind inlines them.
import 'leaflet/dist/leaflet.css'
import '@geoman-io/leaflet-geoman-free/dist/leaflet-geoman.css'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1, // one retry covers a blip; more would hide a real outage behind a long spinner
      staleTime: 5_000,
    },
  },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ToastProvider>
          <LiveAlertsProvider>
            <MapModeProvider>
              <App />
            </MapModeProvider>
          </LiveAlertsProvider>
        </ToastProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </StrictMode>,
)
