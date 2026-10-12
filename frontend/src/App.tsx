import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from './store/authStore';
import { Login } from './pages/Login';
import { Verify2FA } from './pages/Verify2FA';
import { UpdateCredentials } from './pages/UpdateCredentials';
import { SetupSupabaseMFA } from './pages/SetupSupabaseMFA';
import { ChangePassword } from './pages/ChangePassword';
import { DistrictManagers } from './pages/DistrictManagers';
import { MonthlyAnalysis } from './pages/MonthlyAnalysis';
import { DailyAnalysis } from './pages/DailyAnalysis';
import { ProtectedRoute } from './components/ProtectedRoute';
import { DashboardLayout } from './components/DashboardLayout';

function RootRedirect() {
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);
  if (!token || !user) return <Navigate to="/login" replace />;
  return user.role === 'admin' ? <Navigate to="/admin/dashboard" replace /> : <Navigate to="/dm/dashboard" replace />;
}

import { AdminOverview } from './pages/AdminOverview';
import { DMOverview } from './pages/DMOverview';
import { DMUpload } from './pages/DMUpload';
import { Reports } from './pages/Reports';
import { Settings } from './pages/Settings';

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RootRedirect />} />
        <Route path="/login" element={<Login />} />
        <Route path="/verify-2fa" element={<Verify2FA />} />
        <Route
          path="/update-credentials"
          element={
            <ProtectedRoute requireAdmin>
              <UpdateCredentials />
            </ProtectedRoute>
          }
        />
        <Route
          path="/setup-2fa"
          element={
            <ProtectedRoute requireAdmin>
              <SetupSupabaseMFA />
            </ProtectedRoute>
          }
        />
        <Route
          path="/change-password"
          element={
            <ProtectedRoute>
              <ChangePassword />
            </ProtectedRoute>
          }
        />

        {/* Admin Dashboard */}
        <Route
          path="/admin/dashboard"
          element={
            <ProtectedRoute requireAdmin>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<AdminOverview />} />
          <Route path="dms" element={<DistrictManagers />} />
          <Route path="monthly" element={<MonthlyAnalysis />} />
          <Route path="daily" element={<DailyAnalysis />} />
          <Route path="reports" element={<Reports />} />
          <Route path="settings" element={<Settings />} />
        </Route>

        {/* DM Dashboard */}
        <Route
          path="/dm/dashboard"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<DMOverview />} />
          <Route path="upload" element={<DMUpload />} />
          <Route path="monthly" element={<MonthlyAnalysis />} />
          <Route path="daily" element={<DailyAnalysis />} />
          <Route path="reports" element={<Reports />} />
          <Route path="settings" element={<Settings />} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
