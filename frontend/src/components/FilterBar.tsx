import React, { useEffect, useState } from 'react';
import { Filter, Calendar, MapPin, User, RotateCcw, Loader2 } from 'lucide-react';
import { api } from '../lib/api';
import { useFilterStore } from '../store/filterStore';
import { useAuthStore } from '../store/authStore';

interface DistrictOption {
  district_id: number;
  district_name: string;
}

interface DmOption {
  id?: string;
  dm_user_id?: string;
  dmid: string;
  name?: string;
  dm_name?: string;
}

interface FilterOptionsResponse {
  available_months: number[];
  districts: DistrictOption[];
  district_managers: DmOption[];
}

export function FilterBar() {
  const currentUser = useAuthStore((s) => s.user);
  const { month, districtId, dmId, setMonth, setDistrictId, setDmId, resetFilters } = useFilterStore();

  const [options, setOptions] = useState<FilterOptionsResponse>({
    available_months: [],
    districts: [],
    district_managers: [],
  });
  const [loading, setLoading] = useState(false);
  const hasInitialized = React.useRef(false);

  useEffect(() => {
    fetchFilterOptions();
  }, [month, districtId, dmId]);

  const fetchFilterOptions = async () => {
    setLoading(true);
    try {
      const params: any = {};
      if (month) params.month = month;
      if (districtId) params.district_id = districtId;
      if (dmId) params.dm_id = dmId;

      const response = await api.get('/api/filters/options', { params });
      const data: FilterOptionsResponse = response.data;
      setOptions(data);
      hasInitialized.current = true;
    } catch (err) {
      console.error('Failed to load filter options:', err);
    } finally {
      setLoading(false);
    }
  };

  const formatMonthLabel = (m: number) => {
    const s = m.toString();
    if (s.length === 6) {
      const yr = s.substring(0, 4);
      const mo = parseInt(s.substring(4, 6), 10);
      const dateObj = new Date(parseInt(yr, 10), mo - 1, 1);
      return dateObj.toLocaleString('en-US', { month: 'short', year: 'numeric' });
    }
    return s;
  };

  return (
    <div className="bg-white border-b border-slate-200 px-8 py-4 sticky top-0 z-20">
      <div className="flex flex-wrap items-center justify-between gap-4">
        
        {/* Left Label */}
        <div className="flex items-center space-x-3">
          <div className="flex items-center space-x-2 text-[#0F1729] font-bold text-sm tracking-wide">
            <Filter className="w-5 h-5 text-[#0F1729]" />
            <span>Filter Records:</span>
          </div>

          {/* Active Loading Spinner */}
          {loading && (
            <div className="flex items-center space-x-2 text-xs font-semibold text-[#0F1729] border border-blue-200 px-3 py-1.5 rounded-lg animate-pulse">
              <Loader2 className="w-3.5 h-3.5 animate-spin text-[#0F1729]" />
              <span>Updating Data...</span>
            </div>
          )}
        </div>

        {/* Filter Controls */}
        <div className="flex flex-wrap items-center gap-4">
          
          {/* Month Selector */}
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
              <Calendar className="w-4 h-4" />
            </div>
            <select
              value={month || ''}
              onChange={(e) => setMonth(e.target.value ? parseInt(e.target.value, 10) : null)}
              className="pl-10 pr-9 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 cursor-pointer"
            >
              <option value="">All Available Months</option>
              {options.available_months.map((m) => (
                <option key={m} value={m}>
                  {formatMonthLabel(m)}
                </option>
              ))}
            </select>
          </div>

          {/* District Selector */}
          <div className="relative">
            <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
              <MapPin className="w-4 h-4" />
            </div>
            <select
              value={districtId || ''}
              onChange={(e) => setDistrictId(e.target.value ? parseInt(e.target.value, 10) : null)}
              className="pl-10 pr-9 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 cursor-pointer"
            >
              <option value="">All Districts ({options.districts.length})</option>
              {options.districts.map((d) => (
                <option key={d.district_id} value={d.district_id}>
                  {d.district_name}
                </option>
              ))}
            </select>
          </div>

          {/* DM Selector */}
          {currentUser?.role === 'admin' && (
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
                <User className="w-4 h-4" />
              </div>
              <select
                value={dmId || ''}
                onChange={(e) => setDmId(e.target.value || null)}
                className="pl-10 pr-9 py-2.5 border border-slate-300 rounded-lg text-sm font-semibold text-[#0F1729] focus:outline-none focus:ring-2 focus:ring-blue-900 cursor-pointer"
              >
                <option value="">All District Managers</option>
                {options.district_managers.map((dm) => {
                  const val = dm.id || dm.dm_user_id || dm.dmid;
                  const label = dm.name || dm.dm_name || dm.dmid;
                  return (
                    <option key={val} value={val}>
                      {label} ({dm.dmid})
                    </option>
                  );
                })}
              </select>
            </div>
          )}

          {/* Reset Filters Button */}
          {(month || districtId || dmId) && (
            <button
              type="button"
              onClick={resetFilters}
              className="flex items-center space-x-1.5 px-3.5 py-2.5 hover:bg-slate-200 text-[#0F1729] rounded-lg text-sm font-semibold transition border border-slate-200"
              title="Reset all filters to default"
            >
              <RotateCcw className="w-4 h-4" />
              <span>Reset Filters</span>
            </button>
          )}

        </div>

      </div>
    </div>
  );
}
