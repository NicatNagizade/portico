import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import ConnectionsPage from './pages/ConnectionsPage'
import ConnectionFormPage from './pages/ConnectionFormPage'
import ExplorePage from './pages/ExplorePage'
import SyncJobsPage from './pages/SyncJobsPage'
import SyncJobFormPage from './pages/SyncJobFormPage'
import SyncJobDetailPage from './pages/SyncJobDetailPage'
import SyncLogsPage from './pages/SyncLogsPage'
import SyncLogDetailPage from './pages/SyncLogDetailPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Navigate to="/connections" replace />} />
          <Route path="connections" element={<ConnectionsPage />} />
          <Route path="connections/new" element={<ConnectionFormPage />} />
          <Route path="connections/:id/edit" element={<ConnectionFormPage />} />
          <Route path="sync-jobs" element={<SyncJobsPage />} />
          <Route path="sync-jobs/new" element={<SyncJobFormPage />} />
          <Route path="sync-jobs/:id" element={<SyncJobDetailPage />} />
          <Route path="sync-jobs/:id/edit" element={<SyncJobFormPage />} />
          <Route path="explore" element={<ExplorePage />} />
          <Route path="sync-logs" element={<SyncLogsPage />} />
          <Route path="sync-logs/:id" element={<SyncLogDetailPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
