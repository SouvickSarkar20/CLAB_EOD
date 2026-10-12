import React from 'react';
import { useAuthStore } from '../store/authStore';
import { ShieldCheck, CalendarCheck, FileText, ChevronRight, Activity, TrendingUp, AlertCircle, Loader2, UploadCloud } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import { useFilterStore } from '../store/filterStore';
import {
  BarChart,
  Bar,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
  Label
} from 'recharts';

interface DailyTrendPoint {
  enroll_date: string;
  label: string;
  total_enrollment: number;
  total_revenue: number;
}

interface TopStationData {
  station_id: string;
  station_name: string;
  district_name: string;
  active_days: number;
  total_enrollment: number;
  total_revenue: number;
}

interface CategoryMixItem {
  category: string;
  count: number;
}

interface DMOverviewResponse {
  total_enrollment: number;
  total_revenue: number;
  active_days_count: number;
  daily_trend: DailyTrendPoint[];
  top_stations: TopStationData[];
  category_mix: CategoryMixItem[];
}

const COLORS = ['#1E3A8A', '#3B82F6', '#60A5FA', '#93C5FD', '#BFDBFE', '#DBEAFE'];

export function DMOverview() {
  const user = useAuthStore((s) => s.user);
  const { month, districtId, startDate, endDate } = useFilterStore();

  const { data, isLoading, error } = useQuery<DMOverviewResponse>({
    queryKey: ['dm_overview', month, districtId, startDate, endDate],
    queryFn: async () => {
      const params: any = {};
      if (month) params.month = month;
      if (districtId) params.district_id = districtId;
      if (startDate) params.start_date = startDate;
      if (endDate) params.end_date = endDate;
      
      const response = await api.get('/api/dm/overview', { params });
      return response.data;
    },
  });

  return (
    <div className="space-y-6 pb-12 font-sans">
      {/* Welcome Section */}
      <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0] flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div>
          <h1 className="text-[22px] font-bold text-[#0F1729]">
            Welcome back, {user?.name || 'District Manager'}
          </h1>
          <p className="text-[#4B5563] text-[13px] mt-1">
            District ID: <span className="font-mono text-[#0F1729] font-medium">{user?.dmid || 'N/A'}</span> &bull; Operations summary
          </p>
        </div>
        <div className="flex items-center space-x-2 text-[#16A34A] px-3 py-1.5 rounded-[4px] border border-[#16A34A]/20 bg-[#16A34A]/5">
          <ShieldCheck className="w-4 h-4" />
          <span className="font-semibold text-[13px]">District Operational</span>
        </div>
      </div>

      {isLoading && (
        <div className="h-64 flex flex-col items-center justify-center space-y-4">
          <Loader2 className="w-8 h-8 text-[#1A3A8F] animate-spin" />
          <p className="text-[#4B5563] text-[13px] font-medium">Loading district summary...</p>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-[8px] p-6 flex items-start space-x-3">
          <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
          <div>
            <h3 className="font-bold text-red-900 text-[14px]">Failed to load overview data</h3>
            <p className="text-red-700 text-[13px] mt-1">
              There was a problem communicating with the server. Please try again later.
            </p>
          </div>
        </div>
      )}

      {!isLoading && !error && data && (
        <>
          {/* KPI Cards */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-white p-5 rounded-[8px] border border-[#E2E8F0] flex flex-col justify-between relative">
              <Activity className="w-4 h-4 text-[#9CA3AF] absolute top-5 right-5" />
              <span className="text-[#9CA3AF] text-[11px] font-semibold uppercase tracking-wide">Total Enrollment</span>
              <div className="mt-3 text-[28px] font-extrabold text-[#0F1729]">
                {(data.total_enrollment || 0).toLocaleString()}
              </div>
            </div>
            
            <div className="bg-white p-5 rounded-[8px] border border-[#E2E8F0] flex flex-col justify-between relative">
              <TrendingUp className="w-4 h-4 text-[#9CA3AF] absolute top-5 right-5" />
              <span className="text-[#9CA3AF] text-[11px] font-semibold uppercase tracking-wide">Total Revenue</span>
              <div className="mt-3 text-[28px] font-extrabold text-[#0F1729] font-mono">
                ₹{(data.total_revenue || 0).toLocaleString()}
              </div>
            </div>

            <div className="bg-white p-5 rounded-[8px] border border-[#E2E8F0] flex flex-col justify-between relative">
              <CalendarCheck className="w-4 h-4 text-[#9CA3AF] absolute top-5 right-5" />
              <span className="text-[#9CA3AF] text-[11px] font-semibold uppercase tracking-wide">Active Days Recorded</span>
              <div className="mt-3 text-[28px] font-extrabold text-[#0F1729]">
                {data.active_days_count} <span className="text-[14px] font-medium text-[#4B5563]">Days</span>
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Trend Chart */}
            <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0] col-span-1 lg:col-span-2">
              <div className="mb-6">
                <h3 className="font-bold text-[#0F1729] text-[14px]">Daily Enrollment Trend</h3>
                <p className="text-[12px] text-[#4B5563]">Historical enrollment volume trajectory</p>
              </div>
              <div className="h-64">
                {data.daily_trend && data.daily_trend.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={data.daily_trend} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                      <defs>
                        <linearGradient id="colorEnrollment" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#1A3A8F" stopOpacity={0.15}/>
                          <stop offset="95%" stopColor="#1A3A8F" stopOpacity={0}/>
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#E2E8F0" />
                      <XAxis dataKey="label" tick={{ fontSize: 11, fill: '#9CA3AF' }} axisLine={false} tickLine={false} minTickGap={30} />
                      <YAxis tick={{ fontSize: 11, fill: '#9CA3AF' }} axisLine={false} tickLine={false} />
                      <Tooltip cursor={{ stroke: '#CBD5E1', strokeWidth: 1, strokeDasharray: '4 4' }} contentStyle={{ borderRadius: '8px', border: '1px solid #E2E8F0', boxShadow: 'none' }} itemStyle={{ fontSize: '12px' }} />
                      <Area type="monotone" dataKey="total_enrollment" name="Enrollments" stroke="#1A3A8F" strokeWidth={2} fillOpacity={1} fill="url(#colorEnrollment)" activeDot={{ r: 5, fill: '#1A3A8F', stroke: '#FFFFFF', strokeWidth: 2 }} />
                    </AreaChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="h-full flex items-center justify-center text-[13px] italic text-[#9CA3AF]">
                    No trend data available for this period.
                  </div>
                )}
              </div>
            </div>

            {/* Quick Actions & Category Mix */}
            <div className="space-y-6">
              <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0]">
                <h3 className="font-bold text-[#0F1729] text-[14px] mb-4">Quick Actions</h3>
                <div className="space-y-3">
                  <Link to="/dm/dashboard/upload" className="w-full flex items-center justify-between p-3 rounded-[6px] border border-[#E2E8F0] hover:bg-[#F8FAFC] transition">
                    <div className="flex items-center space-x-3 text-[#0F1729]">
                      <UploadCloud className="w-4 h-4 text-[#4B5563]" />
                      <span className="font-medium text-[13px]">Upload EOD Data</span>
                    </div>
                    <ChevronRight className="w-4 h-4 text-[#9CA3AF]" />
                  </Link>
                  <Link to="/dm/dashboard/daily" className="w-full flex items-center justify-between p-3 rounded-[6px] border border-[#E2E8F0] hover:bg-[#F8FAFC] transition">
                    <div className="flex items-center space-x-3 text-[#0F1729]">
                      <FileText className="w-4 h-4 text-[#4B5563]" />
                      <span className="font-medium text-[13px]">View Daily Logs</span>
                    </div>
                    <ChevronRight className="w-4 h-4 text-[#9CA3AF]" />
                  </Link>
                </div>
              </div>

              <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0]">
                <h3 className="font-bold text-[#0F1729] text-[14px] mb-2">Category Breakdown</h3>
                <div className="h-48 w-full flex items-center justify-center">
                  {data.category_mix && data.category_mix.length > 0 ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={data.category_mix}
                          dataKey="count"
                          nameKey="category"
                          cx="50%"
                          cy="50%"
                          innerRadius={45}
                          outerRadius={65}
                          paddingAngle={2}
                          stroke="none"
                          label={(props: any) => {
                            const PIE_COLORS = ['#1D4ED8', '#047857', '#B91C1C', '#6D28D9', '#0369A1', '#A21CAF'];
                            const { x, y, cx, value, index } = props;
                            return (
                              <text
                                x={x}
                                y={y}
                                fill={PIE_COLORS[index % PIE_COLORS.length]}
                                textAnchor={x > cx ? 'start' : 'end'}
                                dominantBaseline="central"
                                fontSize={12}
                                fontWeight={700}
                              >
                                {value.toLocaleString()}
                              </text>
                            );
                          }}
                          labelLine={{ stroke: '#CBD5E1', strokeWidth: 1 }}
                        >
                          {data.category_mix.map((entry, index) => {
                            const PIE_COLORS = ['#1D4ED8', '#047857', '#B91C1C', '#6D28D9', '#0369A1', '#A21CAF'];
                            return <Cell key={`cell-${index}`} fill={PIE_COLORS[index % PIE_COLORS.length]} />;
                          })}
                        </Pie>
                        <Tooltip contentStyle={{ borderRadius: '8px', border: '1px solid #E2E8F0', boxShadow: 'none' }} itemStyle={{ fontSize: '12px' }} />
                        <Legend wrapperStyle={{ fontSize: '11px', color: '#4B5563' }} />
                      </PieChart>
                    </ResponsiveContainer>
                  ) : (
                    <div className="text-[13px] italic text-[#9CA3AF]">No fee category data.</div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Top Stations Table */}
          <div className="bg-white rounded-[8px] border border-[#E2E8F0] overflow-hidden">
            <div className="p-5 border-b border-[#E2E8F0]">
              <h3 className="font-bold text-[#0F1729] text-[14px]">Top Performing Stations</h3>
              <p className="text-[12px] text-[#4B5563]">Highest enrollment contributing stations</p>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-[#F8FAFC] border-b border-[#E2E8F0] text-[11px] font-semibold uppercase tracking-[0.05em] text-[#4B5563]">
                    <th className="py-3 px-5">Station ID</th>
                    <th className="py-3 px-5 text-right">Enrollments</th>
                    <th className="py-3 px-5 text-right">Revenue Generated</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#F1F5F9] text-[13px]">
                  {data.top_stations && data.top_stations.length > 0 ? (
                    data.top_stations.map((station, idx) => (
                      <tr key={idx} className="hover:bg-[#F8FAFC] transition h-[48px]">
                        <td className="py-2 px-5 font-mono text-[#0F1729]">{station.station_id}</td>
                        <td className="py-2 px-5 text-right text-[#0F1729]">{(station.total_enrollment || 0).toLocaleString()}</td>
                        <td className="py-2 px-5 text-right text-[#4B5563] font-mono">₹{(station.total_revenue || 0).toLocaleString()}</td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td colSpan={3} className="py-8 text-center text-[13px] italic text-[#9CA3AF]">
                        No station data available.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
