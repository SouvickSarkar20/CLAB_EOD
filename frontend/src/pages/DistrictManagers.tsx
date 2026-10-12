import React, { useEffect, useState } from 'react';
import {
  Users,
  Plus,
  Search,
  Filter,
  RefreshCw,
  Key,
  Edit2,
  UserX,
  Eye,
  X,
  Check,
  Copy,
  AlertCircle,
  Building2,
  MapPin,
  Mail,
  Shield,
  Loader2,
  ChevronLeft,
  ChevronRight
} from 'lucide-react';
import { api } from '../lib/api';

interface DMListItem {
  id: string;
  dmid: string;
  name: string;
  email: string;
  status: 'active' | 'inactive';
  assigned_stations_count: number;
  assigned_districts: string[];
  last_login_at: string | null;
  created_at: string;
}

interface DMStationAssignment {
  station_id: string;
  station_name?: string;
  district_id: number;
  district_name?: string;
  effective_month: number;
}

interface DMDetail {
  id: string;
  dmid: string;
  name: string;
  email: string;
  status: 'active' | 'inactive';
  assigned_stations: DMStationAssignment[];
  assigned_districts: string[];
  last_login_at: string | null;
  created_at: string;
  updated_at: string;
}

interface DistrictOption {
  district_id: number;
  district_name: string;
}

interface StationOption {
  station_id: string;
  station_name: string;
}

interface ResetPasswordModalData {
  dmid: string;
  dmName: string;
  tempPassword: string;
}

export function DistrictManagers() {
  const [dms, setDms] = useState<DMListItem[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [pageSize] = useState<number>(10);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Filters & Search
  const [search, setSearch] = useState<string>('');
  const [districtFilter, setDistrictFilter] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [districtOptions, setDistrictOptions] = useState<DistrictOption[]>([]);
  const [stationOptions, setStationOptions] = useState<StationOption[]>([]);

  // Modals & Drawer State
  const [isAddModalOpen, setIsAddModalOpen] = useState<boolean>(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState<boolean>(false);
  const [selectedDmForEdit, setSelectedDmForEdit] = useState<DMListItem | null>(null);
  
  // Reset Password Modal State
  const [selectedDmForReset, setSelectedDmForReset] = useState<DMListItem | null>(null);
  const [customResetPassword, setCustomResetPassword] = useState<string>('');
  const [resetSubmitting, setResetSubmitting] = useState<boolean>(false);
  const [resetError, setResetError] = useState<string | null>(null);
  const [resetSuccessData, setResetSuccessData] = useState<{ dmid: string; dmName: string; password: string } | null>(null);
  const [copiedPassword, setCopiedPassword] = useState<boolean>(false);
  
  const [selectedDmDetail, setSelectedDmDetail] = useState<DMDetail | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState<boolean>(false);
  const [drawerLoading, setDrawerLoading] = useState<boolean>(false);

  // Form states
  const [formName, setFormName] = useState<string>('');
  const [formEmail, setFormEmail] = useState<string>('');
  const [formDmid, setFormDmid] = useState<string>('');
  const [formDistrictIds, setFormDistrictIds] = useState<number[]>([]);
  const [formStationIds, setFormStationIds] = useState<string[]>([]);
  const [formStatus, setFormStatus] = useState<'active' | 'inactive'>('active');
  const [formSubmitting, setFormSubmitting] = useState<boolean>(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Fetch DM list & district options on mount / filter change
  useEffect(() => {
    fetchDistrictOptions();
  }, []);

  useEffect(() => {
    fetchDmList();
  }, [page, districtFilter, statusFilter]);

  const fetchDistrictOptions = async () => {
    try {
      const res = await api.get('/api/filters/options');
      setDistrictOptions(res.data.districts || []);
      setStationOptions(res.data.stations || []);
    } catch (err) {
      console.error('Failed to fetch filter options', err);
    }
  };

  const fetchDmList = async () => {
    setLoading(true);
    setError(null);
    try {
      const params: any = {
        page,
        page_size: pageSize,
      };
      if (search.trim()) params.search = search.trim();
      if (districtFilter) params.district_id = parseInt(districtFilter, 10);
      if (statusFilter) params.status = statusFilter;

      const res = await api.get('/api/district-managers/', { params });
      setDms(res.data.items || []);
      setTotal(res.data.total || 0);
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        setError(detail.map((e: any) => e.msg).join(', '));
      } else {
        setError(detail || 'Unable to load District Managers directory.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    fetchDmList();
  };

  const handleResetFilters = () => {
    setSearch('');
    setDistrictFilter('');
    setStatusFilter('');
    setPage(1);
  };

  // Open Add Modal
  const openAddModal = () => {
    setFormName('');
    setFormEmail('');
    setFormDmid('');
    setFormDistrictIds([]);
    setFormStationIds([]);
    setFormError(null);
    setIsAddModalOpen(true);
  };

  // Submit Add DM
  const handleCreateDm = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formName.trim() || !formEmail.trim()) {
      setFormError('Name and Email are required.');
      return;
    }
    setFormSubmitting(true);
    setFormError(null);
    try {
      const payload: any = {
        name: formName.trim(),
        email: formEmail.trim(),
        district_ids: formDistrictIds,
      };
      if (formDmid.trim()) {
        payload.dmid = formDmid.trim();
      }

      const res = await api.post('/api/district-managers/', payload);
      setIsAddModalOpen(false);

      // Prompt generated credentials
      fetchDmList();
      alert(`District Manager '${res.data.name}' created successfully! Assigned DMID: ${res.data.dmid}`);
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        setFormError(detail.map((e: any) => e.msg).join(', '));
      } else {
        setFormError(detail || 'Failed to create District Manager.');
      }
    } finally {
      setFormSubmitting(false);
    }
  };

  // Open Edit Modal
  const openEditModal = async (dm: DMListItem) => {
    setSelectedDmForEdit(dm);
    setFormName(dm.name);
    setFormEmail(dm.email);
    setFormStatus(dm.status);
    setFormDistrictIds([]);
    setFormStationIds([]);
    setFormError(null);
    setIsEditModalOpen(true);

    // Pre-populate with currently assigned stations from Azure SQL WorkingKIT
    try {
      const res = await api.get(`/api/district-managers/${dm.id}`);
      if (res.data?.assigned_stations) {
        const currentIds = res.data.assigned_stations.map((s: any) => s.station_id);
        setFormStationIds(currentIds);
      }
    } catch (err) {
      console.warn("Could not load current DM station assignments", err);
    }
  };

  // Submit Edit DM
  const handleUpdateDm = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDmForEdit) return;

    setFormSubmitting(true);
    setFormError(null);
    try {
      const payload: any = {
        name: formName.trim(),
        email: formEmail.trim(),
        status: formStatus,
        station_ids: formStationIds, // Always send the updated list of stations
      };
      if (formDistrictIds.length > 0) {
        payload.district_ids = formDistrictIds;
      }

      await api.put(`/api/district-managers/${selectedDmForEdit.id}`, payload);
      setIsEditModalOpen(false);
      fetchDmList();
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        setFormError(detail.map((e: any) => e.msg).join(', '));
      } else {
        setFormError(detail || 'Failed to update District Manager details.');
      }
    } finally {
      setFormSubmitting(false);
    }
  };

  const generateRandomTempPw = (length = 12) => {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*';
    let res = '';
    for (let i = 0; i < length; i++) {
      res += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return res;
  };

  // Open Reset Password Modal for DM
  const openResetPasswordModal = (dm: DMListItem) => {
    setSelectedDmForReset(dm);
    setCustomResetPassword(generateRandomTempPw());
    setResetError(null);
    setResetSuccessData(null);
    setCopiedPassword(false);
  };

  // Save Custom / Generated Password for DM
  const handleSaveCustomPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedDmForReset) return;
    if (!customResetPassword || customResetPassword.length < 6) {
      setResetError('Password must be at least 6 characters long.');
      return;
    }

    setResetSubmitting(true);
    setResetError(null);

    try {
      const res = await api.post(`/api/district-managers/${selectedDmForReset.id}/reset-password`, {
        new_password: customResetPassword.trim()
      });

      setResetSuccessData({
        dmid: res.data.dmid,
        dmName: selectedDmForReset.name,
        password: res.data.temporary_password || customResetPassword.trim()
      });
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        setResetError(detail.map((e: any) => e.msg).join(', '));
      } else {
        setResetError(detail || 'Failed to update DM password. Please try again.');
      }
    } finally {
      setResetSubmitting(false);
    }
  };

  // Deactivate DM
  const handleDeactivateDm = async (dm: DMListItem) => {
    if (!window.confirm(`Are you sure you want to deactivate DM ${dm.name} (${dm.dmid})? Their account status will be set to inactive.`)) {
      return;
    }

    try {
      await api.post(`/api/district-managers/${dm.id}/deactivate`);
      fetchDmList();
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        alert(detail.map((e: any) => e.msg).join(', '));
      } else {
        alert(detail || 'Failed to deactivate District Manager.');
      }
    }
  };

  // View DM Details (Slide-over drawer)
  const openDmDrawer = async (dmId: string) => {
    setIsDrawerOpen(true);
    setDrawerLoading(true);
    setSelectedDmDetail(null);
    try {
      const res = await api.get(`/api/district-managers/${dmId}`);
      setSelectedDmDetail(res.data);
    } catch (err: any) {
      const detail = err.response?.data?.detail;
      if (Array.isArray(detail)) {
        alert(detail.map((e: any) => e.msg).join(', '));
      } else {
        alert(detail || 'Failed to load DM details.');
      }
      setIsDrawerOpen(false);
    } finally {
      setDrawerLoading(false);
    }
  };

  // Copy password to clipboard
  const handleCopyPassword = () => {
    if (resetSuccessData?.password) {
      navigator.clipboard.writeText(resetSuccessData.password);
      setCopiedPassword(true);
      setTimeout(() => setCopiedPassword(false), 3000);
    }
  };

  const totalPages = Math.ceil(total / pageSize) || 1;

  return (
    <div className="space-y-8 pb-12">
      
      {/* Page Header */}
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 pb-5">
        <div>
          <h1 className="text-2xl font-extrabold text-[#0F1729] flex items-center space-x-3">
            <Users className="w-7 h-7 text-[#0F1729]" />
            <span>District Managers Directory</span>
          </h1>
          <p className="text-sm text-slate-600 mt-1">
            Onboard, edit, reset security credentials, and supervise District Managers across all operational regions.
          </p>
        </div>

        <button
          type="button"
          onClick={openAddModal}
          className="flex items-center space-x-2 px-5 py-3 bg-[#1A3A8F] text-white font-bold rounded-lg text-sm hover:bg-[#2952C4] transition cursor-pointer"
        >
          <Plus className="w-5 h-5" />
          <span>Add New District Manager</span>
        </button>
      </div>

      {/* Search & Filter Bar */}
      <div className="bg-white p-5 rounded-lg border border-slate-200 space-y-4">
        <form onSubmit={handleSearchSubmit} className="flex flex-wrap items-center gap-4">
          
          {/* Text Search */}
          <div className="relative flex-1 min-w-[260px]">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
              <Search className="w-4 h-4" />
            </div>
            <input
              type="text"
              placeholder="Search DM by Name, Email, or DMID..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-10 pr-4 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-900"
            />
          </div>

          {/* District Filter */}
          <div className="relative min-w-[180px]">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
              <MapPin className="w-4 h-4" />
            </div>
            <select
              value={districtFilter}
              onChange={(e) => {
                setDistrictFilter(e.target.value);
                setPage(1);
              }}
              className="w-full pl-10 pr-8 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 cursor-pointer"
            >
              <option value="">All Districts</option>
              {districtOptions.map((d) => (
                <option key={d.district_id} value={d.district_id}>
                  {d.district_name}
                </option>
              ))}
            </select>
          </div>

          {/* Status Filter */}
          <div className="relative min-w-[150px]">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
              <Filter className="w-4 h-4" />
            </div>
            <select
              value={statusFilter}
              onChange={(e) => {
                setStatusFilter(e.target.value);
                setPage(1);
              }}
              className="w-full pl-10 pr-8 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 cursor-pointer"
            >
              <option value="">All Statuses</option>
              <option value="active">Active Only</option>
              <option value="inactive">Inactive Only</option>
            </select>
          </div>

          {/* Submit Search */}
          <button
            type="submit"
            className="px-4 py-2.5 bg-slate-900 text-white rounded-lg text-sm font-semibold hover:bg-slate-800 transition cursor-pointer"
          >
            Search
          </button>

          {/* Reset Filters */}
          {(search || districtFilter || statusFilter) && (
            <button
              type="button"
              onClick={handleResetFilters}
              className="flex items-center space-x-1 px-3 py-2.5 hover:bg-slate-200 text-[#0F1729] rounded-lg text-sm font-semibold transition border border-slate-200 cursor-pointer"
            >
              <RefreshCw className="w-4 h-4" />
              <span>Reset</span>
            </button>
          )}

        </form>
      </div>

      {/* Error Callout */}
      {error && (
        <div className="border border-red-200 rounded-lg p-5 text-[#0F1729] flex items-center space-x-3">
          <AlertCircle className="w-6 h-6 text-red-600 flex-shrink-0" />
          <div className="text-sm font-medium">{error}</div>
        </div>
      )}

      {/* DM Directory Data Table */}
      <div className="bg-white rounded-lg border border-slate-200 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-200 text-[#4B5563] text-xs font-bold uppercase tracking-wider">
                <th className="py-3.5 px-4">DM Name & ID</th>
                <th className="py-3.5 px-4">Email Address</th>
                <th className="py-3.5 px-4">Assigned Districts</th>
                <th className="py-3.5 px-4 text-center">Supervised Stations</th>
                <th className="py-3.5 px-4 text-center">Status</th>
                <th className="py-3.5 px-4 text-center">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 text-sm">
              {loading ? (
                <tr>
                  <td colSpan={6} className="text-center py-12 text-slate-500">
                    <div className="flex items-center justify-center space-x-2 text-sm font-semibold">
                      <Loader2 className="w-5 h-5 animate-spin text-[#0F1729]" />
                      <span>Loading District Managers...</span>
                    </div>
                  </td>
                </tr>
              ) : dms.length > 0 ? (
                dms.map((dm) => (
                  <tr key={dm.id} className="hover:bg-slate-50 transition">
                    
                    {/* DM Name & ID */}
                    <td className="py-4 px-4">
                      <div className="font-bold text-[#0F1729]">{dm.name}</div>
                      <div className="inline-block mt-0.5 px-2 py-0.5 rounded text-[#4B5563] text-xs font-mono font-bold border border-slate-200">
                        {dm.dmid}
                      </div>
                    </td>

                    {/* Email */}
                    <td className="py-4 px-4 text-[#0F1729] font-medium">
                      <div className="flex items-center space-x-1.5">
                        <Mail className="w-4 h-4 text-slate-400" />
                        <span>{dm.email}</span>
                      </div>
                    </td>

                    {/* Assigned Districts */}
                    <td className="py-4 px-4">
                      {dm.assigned_districts && dm.assigned_districts.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {dm.assigned_districts.map((dist, i) => (
                            <span
                              key={i}
                              className="px-2.5 py-0.5 rounded-md text-xs font-semibold text-[#4B5563] border border-slate-200"
                            >
                              {dist}
                            </span>
                          ))}
                        </div>
                      ) : (
                        <span className="text-xs text-slate-400 italic">Unassigned</span>
                      )}
                    </td>

                    {/* Supervised Stations */}
                    <td className="py-4 px-4 text-center">
                      <span className="inline-flex items-center space-x-1 px-3 py-1 rounded-md text-xs font-bold text-[#4B5563] border border-slate-200">
                        <Building2 className="w-3.5 h-3.5" />
                        <span>{dm.assigned_stations_count} Stations</span>
                      </span>
                    </td>

                    {/* Status */}
                    <td className="py-4 px-4 text-center">
                      {dm.status === 'active' ? (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-bold bg-emerald-50 text-emerald-700 border border-emerald-200">
                          Active
                        </span>
                      ) : (
                        <span className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-bold bg-red-50 text-red-700 border border-red-200">
                          Inactive
                        </span>
                      )}
                    </td>

                    {/* Actions */}
                    <td className="py-4 px-4">
                      <div className="flex flex-wrap items-center justify-center gap-1.5">
                        
                        {/* View Stations */}
                        <button
                          type="button"
                          onClick={() => openDmDrawer(dm.id)}
                          className="flex items-center space-x-1 px-2.5 py-1.5 text-[#4B5563] hover:text-[#0F1729] rounded-lg text-xs font-semibold border border-slate-200 hover:border-slate-300 transition cursor-pointer"
                          title="View Station Assignments"
                        >
                          <Eye className="w-3.5 h-3.5" />
                          <span>View Stations</span>
                        </button>

                        {/* Reset Password */}
                        <button
                          type="button"
                          onClick={() => openResetPasswordModal(dm)}
                          className="flex items-center space-x-1 px-2.5 py-1.5 text-[#4B5563] hover:text-[#0F1729] rounded-lg text-xs font-semibold border border-slate-200 hover:border-slate-300 transition cursor-pointer"
                          title="Set or generate password for DM account"
                        >
                          <Key className="w-3.5 h-3.5" />
                          <span>Reset Password</span>
                        </button>

                        {/* Edit Profile */}
                        <button
                          type="button"
                          onClick={() => openEditModal(dm)}
                          className="flex items-center space-x-1 px-2.5 py-1.5 text-[#4B5563] hover:bg-slate-200 hover:text-[#0F1729] rounded-lg text-xs font-semibold border border-slate-300 transition cursor-pointer"
                          title="Edit Name, Email, or Status"
                        >
                          <Edit2 className="w-3.5 h-3.5" />
                          <span>Edit</span>
                        </button>

                        {/* Deactivate Account */}
                        {dm.status === 'active' && (
                          <button
                            type="button"
                            onClick={() => handleDeactivateDm(dm)}
                            className="flex items-center space-x-1 px-2.5 py-1.5 text-red-600 hover:text-red-700 rounded-lg text-xs font-semibold border border-red-200 hover:border-red-300 transition cursor-pointer"
                            title="Deactivate District Manager Account"
                          >
                            <UserX className="w-3.5 h-3.5" />
                            <span>Deactivate</span>
                          </button>
                        )}

                      </div>
                    </td>

                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={6} className="text-center py-12 text-slate-400 text-sm">
                    No District Managers found matching search criteria.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Bar */}
        <div className="border-t border-slate-200 px-6 py-4 flex items-center justify-between text-xs text-slate-600 font-semibold">
          <div>
            Showing {total > 0 ? (page - 1) * pageSize + 1 : 0} to {Math.min(page * pageSize, total)} of {total} DMs
          </div>
          <div className="flex items-center space-x-2">
            <button
              type="button"
              disabled={page <= 1}
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              className="p-2 bg-white border border-slate-300 rounded-lg hover: disabled:opacity-40 disabled:cursor-not-allowed transition cursor-pointer"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span>Page {page} of {totalPages}</span>
            <button
              type="button"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
              className="p-2 bg-white border border-slate-300 rounded-lg hover: disabled:opacity-40 disabled:cursor-not-allowed transition cursor-pointer"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>

      </div>

      {/* Add DM Modal */}
      {isAddModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-xs">
          <div className="bg-white rounded-lg border border-slate-200 max-w-lg w-full overflow-hidden p-6 space-y-6">
            <div className="flex items-center justify-between border-b border-slate-100 pb-4">
              <h3 className="text-lg font-bold text-[#0F1729]">Add New District Manager</h3>
              <button
                type="button"
                onClick={() => setIsAddModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 p-1 rounded-lg cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {formError && (
              <div className="border border-red-200 text-[#0F1729] text-xs p-3 rounded-lg">
                {formError}
              </div>
            )}

            <form onSubmit={handleCreateDm} className="space-y-4 text-sm">
              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Full Name <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Rahul Sharma"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                />
              </div>

              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Email Address <span className="text-red-500">*</span>
                </label>
                <input
                  type="email"
                  required
                  placeholder="e.g. rahul.sharma@eod.gov.in"
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                />
              </div>

              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  DMID (Optional)
                </label>
                <input
                  type="text"
                  placeholder="e.g. ClabDM06 (Leave blank to auto-generate)"
                  value={formDmid}
                  onChange={(e) => setFormDmid(e.target.value)}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 font-mono"
                />
              </div>

              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Assign District
                </label>
                <select
                  value={formDistrictIds[0] || ''}
                  onChange={(e) => setFormDistrictIds(e.target.value ? [parseInt(e.target.value, 10)] : [])}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                >
                  <option value="">Select Primary District</option>
                  {districtOptions.map((d) => (
                    <option key={d.district_id} value={d.district_id}>
                      {d.district_name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex items-center justify-end space-x-3 pt-4 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setIsAddModalOpen(false)}
                  className="px-4 py-2 hover:bg-slate-200 text-[#0F1729] font-semibold rounded-lg text-xs cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={formSubmitting}
                  className="px-5 py-2 bg-[#1A3A8F] hover:bg-[#2952C4] text-white font-bold rounded-lg text-xs transition disabled:opacity-50 cursor-pointer"
                >
                  {formSubmitting ? 'Creating...' : 'Create Account'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit DM Modal */}
      {isEditModalOpen && selectedDmForEdit && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-xs">
          <div className="bg-white rounded-lg border border-slate-200 max-w-lg w-full overflow-hidden p-6 space-y-6">
            <div className="flex items-center justify-between border-b border-slate-100 pb-4">
              <h3 className="text-lg font-bold text-[#0F1729]">Edit DM Details - {selectedDmForEdit.dmid}</h3>
              <button
                type="button"
                onClick={() => setIsEditModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 p-1 rounded-lg cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {formError && (
              <div className="border border-red-200 text-[#0F1729] text-xs p-3 rounded-lg">
                {formError}
              </div>
            )}

            <form onSubmit={handleUpdateDm} className="space-y-4 text-sm">
              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Full Name
                </label>
                <input
                  type="text"
                  required
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                />
              </div>

              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Email Address
                </label>
                <input
                  type="email"
                  required
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                />
              </div>

              <div>
                <label className="block text-xs font-bold uppercase text-[#0F1729] mb-1">
                  Account Status
                </label>
                <select
                  value={formStatus}
                  onChange={(e) => setFormStatus(e.target.value as 'active' | 'inactive')}
                  className="w-full px-3.5 py-2 border border-slate-300 rounded-lg text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900"
                >
                  <option value="active">Active</option>
                  <option value="inactive">Inactive</option>
                </select>
              </div>

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label className="block text-xs font-bold uppercase text-[#0F1729]">
                    Assign Supervised Stations ({formStationIds.length} Selected)
                  </label>
                  <div className="flex items-center space-x-2 text-[11px]">
                    <button
                      type="button"
                      onClick={() => setFormStationIds(stationOptions.map(s => s.station_id))}
                      className="text-blue-700 hover:underline font-semibold cursor-pointer"
                    >
                      Select All
                    </button>
                    <span className="text-slate-300">|</span>
                    <button
                      type="button"
                      onClick={() => setFormStationIds([])}
                      className="text-red-600 hover:underline font-semibold cursor-pointer"
                    >
                      Clear
                    </button>
                  </div>
                </div>

                <div className="border border-slate-300 rounded-lg max-h-40 overflow-y-auto p-2 bg-slate-50 space-y-1">
                  {stationOptions.length === 0 ? (
                    <div className="text-center py-4 text-xs text-slate-400">Loading stations...</div>
                  ) : (
                    stationOptions.map((st) => {
                      const isSelected = formStationIds.includes(st.station_id);
                      return (
                        <label
                          key={st.station_id}
                          className={`flex items-center space-x-2.5 px-2.5 py-1.5 rounded-md text-xs cursor-pointer transition select-none ${
                            isSelected
                              ? 'bg-blue-100 text-blue-900 font-bold border border-blue-200'
                              : 'bg-white hover:bg-slate-100 text-slate-700 border border-transparent'
                          }`}
                        >
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setFormStationIds((prev) => [...prev, st.station_id]);
                              } else {
                                setFormStationIds((prev) => prev.filter((id) => id !== st.station_id));
                              }
                            }}
                            className="w-3.5 h-3.5 text-blue-600 rounded border-slate-300 focus:ring-blue-500 cursor-pointer"
                          />
                          <span className="font-mono">{st.station_id}</span>
                          <span className="text-slate-500 font-normal">({st.station_name || `Station ${st.station_id}`})</span>
                        </label>
                      );
                    })
                  )}
                </div>
                <p className="text-[10px] text-slate-500 mt-1">Click any station to assign or unassign it for this District Manager in Azure SQL.</p>
              </div>

              <div className="flex items-center justify-end space-x-3 pt-4 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setIsEditModalOpen(false)}
                  className="px-4 py-2 hover:bg-slate-200 text-[#0F1729] font-semibold rounded-lg text-xs cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={formSubmitting}
                  className="px-5 py-2 bg-[#1A3A8F] hover:bg-[#2952C4] text-white font-bold rounded-lg text-xs transition disabled:opacity-50 cursor-pointer"
                >
                  {formSubmitting ? 'Saving...' : 'Save Changes'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Reset & Set Custom Password Modal */}
      {selectedDmForReset && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-xs animate-fade-in">
          <div className="bg-white rounded-lg border border-slate-200 max-w-lg w-full overflow-hidden p-6 space-y-5">
            
            {/* Modal Header */}
            <div className="flex items-center justify-between border-b border-slate-100 pb-4">
              <div className="flex items-center space-x-3">
                <div className="p-2.5 text-[#9CA3AF] rounded-lg">
                  <Key className="w-6 h-6" />
                </div>
                <div>
                  <h3 className="text-lg font-extrabold text-[#0F1729]">Set Account Password</h3>
                  <p className="text-xs text-slate-500">
                    Reset or assign custom password for <strong>{selectedDmForReset.name}</strong> ({selectedDmForReset.dmid})
                  </p>
                </div>
              </div>

              <button
                type="button"
                onClick={() => setSelectedDmForReset(null)}
                className="text-slate-400 hover:text-slate-600 p-1 rounded-lg cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* If password has been saved successfully */}
            {resetSuccessData ? (
              <div className="space-y-4 text-center py-2">
                <div className="w-12 h-12 text-[#9CA3AF] rounded-md flex items-center justify-center mx-auto">
                  <Check className="w-6 h-6 text-[#0F1729]" />
                </div>
                <div>
                  <h4 className="text-base font-bold text-[#0F1729]">Password Saved Successfully!</h4>
                  <p className="text-xs text-slate-600 mt-0.5">
                    The account password for <strong>{resetSuccessData.dmName}</strong> ({resetSuccessData.dmid}) has been updated.
                  </p>
                </div>

                <div className="p-4 rounded-lg border border-slate-300 flex items-center justify-between font-mono text-sm font-bold text-[#0F1729]">
                  <span className="select-all">{resetSuccessData.password}</span>
                  <button
                    type="button"
                    onClick={handleCopyPassword}
                    className="flex items-center space-x-1.5 text-xs px-3 py-1.5 bg-[#1A3A8F] text-white rounded-lg hover:bg-[#2952C4] transition cursor-pointer"
                  >
                    {copiedPassword ? (
                      <>
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                        <span>Copied!</span>
                      </>
                    ) : (
                      <>
                        <Copy className="w-3.5 h-3.5" />
                        <span>Copy</span>
                      </>
                    )}
                  </button>
                </div>

                <p className="text-xs text-slate-600 p-3 rounded-lg border border-amber-200 text-left">
                  Share this password with the District Manager. They can log in immediately using their email address or DMID.
                </p>

                <button
                  type="button"
                  onClick={() => setSelectedDmForReset(null)}
                  className="w-full py-2.5 bg-slate-900 text-white font-bold rounded-lg text-xs hover:bg-slate-800 transition cursor-pointer"
                >
                  Done & Close
                </button>
              </div>
            ) : (
              /* Editable Form */
              <form onSubmit={handleSaveCustomPassword} className="space-y-4">
                
                {resetError && (
                  <div className="border border-red-200 text-[#0F1729] text-xs p-3 rounded-lg flex items-center space-x-2">
                    <AlertCircle className="w-4 h-4 text-red-600 flex-shrink-0" />
                    <span>{resetError}</span>
                  </div>
                )}

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between">
                    <label className="block text-xs font-bold uppercase tracking-wider text-[#0F1729]">
                      New Password for DM
                    </label>
                    <button
                      type="button"
                      onClick={() => setCustomResetPassword(generateRandomTempPw())}
                      className="text-xs font-semibold text-[#0F1729] hover:underline flex items-center space-x-1 cursor-pointer"
                    >
                      <RefreshCw className="w-3 h-3" />
                      <span>Generate Random</span>
                    </button>
                  </div>

                  <input
                    type="text"
                    required
                    minLength={6}
                    value={customResetPassword}
                    onChange={(e) => setCustomResetPassword(e.target.value)}
                    placeholder="Enter custom password..."
                    className="w-full px-4 py-2.5 border border-slate-300 rounded-lg text-[#0F1729] font-mono text-sm font-bold focus:outline-none focus:ring-2 focus:ring-blue-900"
                  />
                  <p className="text-[11px] text-slate-500">
                    You can type any custom password or click <strong>Generate Random</strong>.
                  </p>
                </div>

                <div className="flex items-center justify-end space-x-3 pt-3 border-t border-slate-100">
                  <button
                    type="button"
                    onClick={() => setSelectedDmForReset(null)}
                    className="px-4 py-2.5 hover:bg-slate-200 text-[#0F1729] font-bold text-xs rounded-lg transition cursor-pointer"
                  >
                    Cancel
                  </button>

                  <button
                    type="submit"
                    disabled={resetSubmitting}
                    className="flex items-center space-x-2 px-5 py-2.5 bg-amber-600 hover:bg-amber-700 text-white font-bold text-xs rounded-lg -xs transition disabled:opacity-50 cursor-pointer"
                  >
                    {resetSubmitting ? (
                      <>
                        <Loader2 className="w-4 h-4 animate-spin" />
                        <span>Saving Password...</span>
                      </>
                    ) : (
                      <>
                        <Key className="w-4 h-4" />
                        <span>Save & Set Password</span>
                      </>
                    )}
                  </button>
                </div>

              </form>
            )}

          </div>
        </div>
      )}

      {/* DM Detail Slide-Over Drawer */}
      {isDrawerOpen && (
        <div className="fixed inset-0 z-50 flex justify-end bg-slate-900/40 backdrop-blur-xs">
          <div className="bg-white w-full max-w-xl h-full overflow-y-auto p-6 space-y-6 flex flex-col justify-between">
            
            <div className="space-y-6">
              {/* Drawer Header */}
              <div className="flex items-center justify-between border-b border-slate-200 pb-4">
                <div className="flex items-center space-x-3">
                  <div className="p-2.5 text-[#9CA3AF] rounded-lg">
                    <Shield className="w-6 h-6" />
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-[#0F1729]">
                      {drawerLoading ? 'Loading Profile...' : selectedDmDetail?.name}
                    </h3>
                    <p className="text-xs text-slate-500 font-mono font-semibold">
                      {selectedDmDetail?.dmid}
                    </p>
                  </div>
                </div>

                <button
                  type="button"
                  onClick={() => setIsDrawerOpen(false)}
                  className="p-2 text-slate-400 hover:text-slate-600 rounded-lg hover: cursor-pointer"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {drawerLoading ? (
                <div className="flex items-center justify-center py-20 text-slate-500 text-sm font-semibold">
                  <Loader2 className="w-6 h-6 animate-spin text-[#0F1729] mr-2" />
                  <span>Loading District Manager details...</span>
                </div>
              ) : selectedDmDetail ? (
                <div className="space-y-6">
                  
                  {/* Info Cards */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="p-4 rounded-lg border border-slate-200 space-y-1">
                      <span className="text-xs font-bold uppercase text-slate-500">Email Address</span>
                      <div className="text-sm font-semibold text-[#0F1729] truncate">
                        {selectedDmDetail.email}
                      </div>
                    </div>

                    <div className="p-4 rounded-lg border border-slate-200 space-y-1">
                      <span className="text-xs font-bold uppercase text-slate-500">Status</span>
                      <div>
                        {selectedDmDetail.status === 'active' ? (
                          <span className="inline-block px-2.5 py-0.5 rounded-md text-xs font-bold text-[#9CA3AF]">
                            Active
                          </span>
                        ) : (
                          <span className="inline-block px-2.5 py-0.5 rounded-md text-xs font-bold bg-slate-200 text-[#0F1729]">
                            Inactive
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Supervised Stations List */}
                  <div className="space-y-3">
                    <div className="flex items-center justify-between">
                      <h4 className="text-sm font-bold text-[#0F1729] flex items-center space-x-2">
                        <Building2 className="w-4 h-4 text-[#0F1729]" />
                        <span>Supervised Operational Stations</span>
                      </h4>
                      <span className="text-xs font-semibold px-2 py-0.5 text-[#9CA3AF] rounded-md border border-blue-200">
                        {selectedDmDetail.assigned_stations.length} Total
                      </span>
                    </div>

                    {selectedDmDetail.assigned_stations.length > 0 ? (
                      <div className="border border-slate-200 rounded-lg overflow-hidden divide-y divide-slate-100 max-h-80 overflow-y-auto">
                        {selectedDmDetail.assigned_stations.map((st, i) => (
                          <div key={i} className="p-3 bg-white hover: flex items-center justify-between text-xs">
                            <div>
                              <div className="font-bold text-[#0F1729]">{st.station_name || st.station_id}</div>
                              <div className="text-slate-500 font-mono">{st.station_id}</div>
                            </div>
                            <div className="text-right">
                              <span className="font-semibold text-[#0F1729]">{st.district_name || 'N/A'}</span>
                              <div className="text-slate-400 text-[10px]">Effective: {st.effective_month}</div>
                            </div>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="p-6 border border-slate-200 rounded-lg text-center text-xs text-slate-400">
                        No individual station assignments recorded.
                      </div>
                    )}
                  </div>

                </div>
              ) : null}
            </div>

            <div className="pt-4 border-t border-slate-200">
              <button
                type="button"
                onClick={() => setIsDrawerOpen(false)}
                className="w-full py-2.5 bg-slate-900 text-white font-bold rounded-lg text-xs hover:bg-slate-800 transition cursor-pointer"
              >
                Close Drawer
              </button>
            </div>

          </div>
        </div>
      )}

    </div>
  );
}

export default DistrictManagers;
