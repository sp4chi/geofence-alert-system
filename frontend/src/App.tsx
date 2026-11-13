import { NavLink, Navigate, Outlet, Route, Routes, useLocation } from 'react-router'
import { AlertFeed } from './components/AlertFeed'
import { ConnectionPill } from './components/ConnectionPill'
import { MapView } from './components/map/MapView'
import { AlertRulesPage } from './pages/AlertRulesPage'
import { GeofencesPage } from './pages/GeofencesPage'
import { HistoryPage } from './pages/HistoryPage'
import { VehiclesPage } from './pages/VehiclesPage'

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Navigate to="/geofences" replace />} />
        <Route path="geofences" element={<GeofencesPage />} />
        <Route path="vehicles" element={<VehiclesPage />} />
        <Route path="alerts" element={<AlertRulesPage />} />
        <Route path="history" element={<HistoryPage />} />
        <Route path="*" element={<Navigate to="/geofences" replace />} />
      </Route>
    </Routes>
  )
}

const NAV = [
  { to: '/geofences', label: 'Geofences' },
  { to: '/vehicles', label: 'Vehicles' },
  { to: '/alerts', label: 'Alert rules' },
  { to: '/history', label: 'History' },
]

/**
 * Desktop: [page panel | map | live alerts]. History needs width for its table, so it takes the
 * map's place (the map stays mounted, just hidden, to keep its position).
 * Mobile: map on top, then the page, then the alert feed, in one scrolling column.
 */
function Layout() {
  const { pathname } = useLocation()
  const isHistory = pathname.startsWith('/history')

  return (
    <div className="flex min-h-full flex-col lg:h-full">
      <header className="flex items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-2.5">
        <h1 className="flex items-center gap-2 text-base font-semibold">
          <img src="/favicon.svg" alt="" className="size-5" />
          Geofence Monitor
        </h1>
        <ConnectionPill />
      </header>

      <div className="flex flex-1 flex-col lg:min-h-0 lg:flex-row">
        <main
          className={`order-first h-[45vh] min-h-72 border-slate-200 lg:order-none lg:h-auto lg:flex-1 lg:border-l ${
            isHistory ? 'hidden' : ''
          }`}
        >
          <MapView visible={!isHistory} />
        </main>

        <aside className={`flex flex-col bg-slate-50 lg:order-first lg:min-h-0 ${isHistory ? 'lg:flex-1' : 'lg:w-[400px]'}`}>
          <nav className="flex gap-1 overflow-x-auto border-b border-slate-200 bg-white px-2" aria-label="Sections">
            {NAV.map(({ to, label }) => (
              <NavLink
                key={to}
                to={to}
                className={({ isActive }) =>
                  `whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-medium ${
                    isActive ? 'border-slate-900 text-slate-900' : 'border-transparent text-slate-500 hover:text-slate-800'
                  }`
                }
              >
                {label}
              </NavLink>
            ))}
          </nav>
          <div className="p-4 lg:flex-1 lg:overflow-y-auto">
            <Outlet />
          </div>
        </aside>

        <div className="flex max-h-[60vh] flex-col border-t border-slate-200 bg-white lg:max-h-none lg:w-80 lg:border-t-0 lg:border-l">
          <AlertFeed />
        </div>
      </div>
    </div>
  )
}
