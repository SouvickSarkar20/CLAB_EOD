import React, { useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  UploadCloud,
  FileText,
  CheckCircle,
  AlertTriangle,
  AlertCircle,
  Loader2,
  Calendar,
  Layers,
  ArrowRight,
  TrendingUp,
  MapPin,
  RefreshCw
} from 'lucide-react';
import { api } from '../lib/api';
import { useAuthStore } from '../store/authStore';
import { useFilterStore } from '../store/filterStore';
import { useQueryClient } from '@tanstack/react-query';

interface PreviewResponse {
  is_valid: boolean;
  total_records: number;
  total_revenue: number;
  target_month: string;
  stations: string[];
  date_range: {
    start: string;
    end: string;
  };
  dm_name_in_file: string;
  dm_name_logged_in: string;
  dm_matches: boolean;
  category_mix: Array<{ category: string; count: number }>;
  sample_rows: any[];
  records: any[];
}

export function DMUpload() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAuthStore((s) => s.user);
  const setMonth = useFilterStore((s) => s.setMonth);

  const [file, setFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [isParsing, setIsParsing] = useState(false);
  const [isCommitting, setIsCommitting] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [previewData, setPreviewData] = useState<PreviewResponse | null>(null);
  const [commitSuccess, setCommitSuccess] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      processSelectedFile(e.dataTransfer.files[0]);
    }
  };

  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      processSelectedFile(e.target.files[0]);
    }
  };

  const processSelectedFile = async (selectedFile: File) => {
    const ext = selectedFile.name.split('.').pop()?.toLowerCase();
    if (ext !== 'txt' && ext !== 'csv') {
      setErrorMsg('Please upload a valid .txt or .csv EOD daily work file.');
      return;
    }

    setFile(selectedFile);
    setErrorMsg(null);
    setPreviewData(null);
    setCommitSuccess(null);
    setIsParsing(true);

    const formData = new FormData();
    formData.append('file', selectedFile);

    try {
      const response = await api.post<PreviewResponse>('/api/dm/upload/preview', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      });
      setPreviewData(response.data);
    } catch (err: any) {
      const detail = err.response?.data?.detail || 'Failed to parse file. Ensure it contains the standard 28-column format.';
      setErrorMsg(detail);
    } finally {
      setIsParsing(false);
    }
  };

  const handleCommit = async () => {
    if (!previewData || !previewData.records || previewData.records.length === 0) return;

    setIsCommitting(true);
    setErrorMsg(null);

    try {
      const res = await api.post('/api/dm/upload/commit', {
        records: previewData.records,
      });
      setCommitSuccess(res.data.message || `Successfully committed ${res.data.inserted_count} records.`);
      
      // Auto-switch filter to newly uploaded month & invalidate queries
      if (previewData.target_month) {
        const parsedM = parseInt(previewData.target_month, 10);
        if (!isNaN(parsedM)) {
          setMonth(parsedM);
        }
      }
      queryClient.invalidateQueries({ queryKey: ['dm_overview'] });

      setTimeout(() => {
        navigate('/dm/dashboard');
      }, 1500);
    } catch (err: any) {
      const detail = err.response?.data?.detail || 'Database commit failed. Please check network/database status.';
      setErrorMsg(detail);
    } finally {
      setIsCommitting(false);
    }
  };

  const handleReset = () => {
    setFile(null);
    setPreviewData(null);
    setErrorMsg(null);
    setCommitSuccess(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  return (
    <div className="space-y-6 pb-16 font-sans max-w-7xl mx-auto">
      {/* Top Header */}
      <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0] flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div>
          <h1 className="text-[22px] font-bold text-[#0F1729]">
            Daily EOD Work File Upload
          </h1>
          <p className="text-[#4B5563] text-[13px] mt-1">
            Upload your daily Aadhaar enrollment & update text file to synchronize your operational records.
          </p>
        </div>
        {previewData && (
          <button
            onClick={handleReset}
            className="flex items-center space-x-2 text-[#4B5563] hover:text-[#0F1729] px-3 py-1.5 rounded-[4px] border border-[#E2E8F0] hover:bg-slate-50 transition text-[13px] font-medium"
          >
            <RefreshCw className="w-4 h-4" />
            <span>Upload Another File</span>
          </button>
        )}
      </div>

      {/* Error Banner */}
      {errorMsg && (
        <div className="bg-red-50 border border-red-200 rounded-[8px] p-5 flex items-start space-x-3 text-red-900 animate-fadeIn">
          <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
          <div className="text-[13px]">
            <span className="font-bold">File Verification Issue: </span>
            {errorMsg}
          </div>
        </div>
      )}

      {/* Success Notification */}
      {commitSuccess && (
        <div className="bg-emerald-50 border border-emerald-200 rounded-[8px] p-5 flex items-start space-x-3 text-emerald-900 animate-fadeIn">
          <CheckCircle className="w-5 h-5 text-emerald-600 flex-shrink-0 mt-0.5" />
          <div className="text-[13px]">
            <span className="font-bold">Records Synchronized! </span>
            {commitSuccess} Redirecting to your Executive Overview...
          </div>
        </div>
      )}

      {/* Upload Dropzone (Shown when no preview is loaded) */}
      {!previewData && (
        <div className="bg-white p-8 rounded-[8px] border border-[#E2E8F0] shadow-sm">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileInputChange}
            accept=".txt,.csv"
            className="hidden"
          />

          <div
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
            className={`border-2 border-dashed rounded-[8px] p-12 flex flex-col items-center justify-center text-center cursor-pointer transition-colors ${
              isDragging
                ? 'border-[#1A3A8F] bg-blue-50/50'
                : 'border-[#CBD5E1] hover:border-[#1A3A8F] hover:bg-slate-50/60'
            }`}
          >
            <div className="w-16 h-16 rounded-full bg-[#1A3A8F]/10 flex items-center justify-center mb-4 text-[#1A3A8F]">
              {isParsing ? (
                <Loader2 className="w-8 h-8 animate-spin" />
              ) : (
                <UploadCloud className="w-8 h-8" />
              )}
            </div>

            <h3 className="text-[16px] font-bold text-[#0F1729]">
              {isParsing ? 'Validating and Parsing EOD File...' : 'Click to select or drag & drop your daily file'}
            </h3>
            <p className="text-[13px] text-[#64748B] mt-1 max-w-md">
              Supports standard 28-column comma-separated <span className="font-mono text-[#0F1729]">.txt</span> or <span className="font-mono text-[#0F1729]">.csv</span> files.
            </p>

            <div className="mt-4 flex items-center space-x-2 text-[12px] text-[#94A3B8]">
              <FileText className="w-4 h-4" />
              <span>Aadhaar EOD Format (SLNO, ENROLMENT_NO_DATE, StationID, etc.)</span>
            </div>
          </div>
        </div>
      )}

      {/* Preview Section */}
      {previewData && (
        <div className="space-y-6">
          {/* Identity & Match Validation Alert */}
          {!previewData.dm_matches && (
            <div className="bg-amber-50 border border-amber-300 rounded-[8px] p-4 flex items-start space-x-3 text-amber-900">
              <AlertTriangle className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
              <div className="text-[13px]">
                <span className="font-bold">Manager Identity Warning: </span>
                The manager name in this file (<span className="font-mono font-bold text-amber-950">{previewData.dm_name_in_file}</span>) does not match your active account (<span className="font-mono font-bold text-amber-950">{previewData.dm_name_logged_in}</span>). Please confirm before committing.
              </div>
            </div>
          )}

          {/* Validation Summary Header Card */}
          <div className="bg-white p-6 rounded-[8px] border border-[#E2E8F0] shadow-sm">
            <div className="flex flex-col md:flex-row md:items-center justify-between pb-4 border-b border-[#E2E8F0] gap-4">
              <div>
                <div className="flex items-center space-x-2">
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-semibold bg-blue-100 text-[#1A3A8F]">
                    Validation Passed
                  </span>
                  <span className="text-[13px] text-[#64748B] font-mono">
                    {file?.name} ({(file ? file.size / 1024 : 0).toFixed(1)} KB)
                  </span>
                </div>
                <h2 className="text-[18px] font-bold text-[#0F1729] mt-2">
                  Pre-Commit Data Verification
                </h2>
              </div>

              {/* Action Buttons */}
              <div className="flex items-center space-x-3">
                <button
                  type="button"
                  onClick={handleReset}
                  disabled={isCommitting}
                  className="px-4 py-2 border border-[#CBD5E1] rounded-[6px] text-[13px] font-semibold text-[#475569] hover:bg-slate-50 transition disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleCommit}
                  disabled={isCommitting || !previewData.is_valid}
                  className="flex items-center space-x-2 bg-[#1A3A8F] hover:bg-[#152e72] text-white px-5 py-2 rounded-[6px] text-[13px] font-semibold transition shadow-sm disabled:opacity-50"
                >
                  {isCommitting ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Saving to Database...</span>
                    </>
                  ) : (
                    <>
                      <span>Commit {previewData.total_records.toLocaleString()} Records</span>
                      <ArrowRight className="w-4 h-4" />
                    </>
                  )}
                </button>
              </div>
            </div>

            {/* Metrics Grid */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-6">
              <div className="bg-slate-50 p-4 rounded-[6px] border border-slate-100">
                <div className="flex items-center space-x-2 text-[11px] font-semibold text-[#64748B] uppercase tracking-wide">
                  <Layers className="w-4 h-4 text-[#1A3A8F]" />
                  <span>Total Records</span>
                </div>
                <div className="text-[24px] font-extrabold text-[#0F1729] mt-2 font-mono">
                  {previewData.total_records.toLocaleString()}
                </div>
              </div>

              <div className="bg-slate-50 p-4 rounded-[6px] border border-slate-100">
                <div className="flex items-center space-x-2 text-[11px] font-semibold text-[#64748B] uppercase tracking-wide">
                  <TrendingUp className="w-4 h-4 text-emerald-600" />
                  <span>Total Revenue</span>
                </div>
                <div className="text-[24px] font-extrabold text-[#0F1729] mt-2 font-mono">
                  ₹{previewData.total_revenue.toLocaleString()}
                </div>
              </div>

              <div className="bg-slate-50 p-4 rounded-[6px] border border-slate-100">
                <div className="flex items-center space-x-2 text-[11px] font-semibold text-[#64748B] uppercase tracking-wide">
                  <Calendar className="w-4 h-4 text-blue-600" />
                  <span>Date Range / Month</span>
                </div>
                <div className="text-[13px] font-bold text-[#0F1729] mt-2">
                  {previewData.date_range.start} &rarr; {previewData.date_range.end}
                </div>
                <div className="text-[11px] text-[#64748B] mt-0.5">
                  Enroll Month: <span className="font-mono text-[#0F1729]">{previewData.target_month}</span>
                </div>
              </div>

              <div className="bg-slate-50 p-4 rounded-[6px] border border-slate-100">
                <div className="flex items-center space-x-2 text-[11px] font-semibold text-[#64748B] uppercase tracking-wide">
                  <MapPin className="w-4 h-4 text-purple-600" />
                  <span>Stations Detected</span>
                </div>
                <div className="text-[24px] font-extrabold text-[#0F1729] mt-2 font-mono">
                  {previewData.stations.length}
                </div>
                <div className="text-[11px] text-[#64748B] mt-0.5 truncate" title={previewData.stations.join(', ')}>
                  IDs: {previewData.stations.slice(0, 3).join(', ')}{previewData.stations.length > 3 ? '...' : ''}
                </div>
              </div>
            </div>

            {/* Category Breakdown Badges */}
            <div className="mt-6 pt-5 border-t border-[#E2E8F0]">
              <h4 className="text-[12px] font-semibold text-[#64748B] uppercase tracking-wide mb-3">
                Update Type Distribution
              </h4>
              <div className="flex flex-wrap gap-2">
                {previewData.category_mix.map((item, idx) => (
                  <div
                    key={idx}
                    className="inline-flex items-center px-3 py-1.5 rounded-[4px] bg-[#F1F5F9] border border-[#E2E8F0] text-[12px]"
                  >
                    <span className="font-semibold text-[#1E293B] mr-2">{item.category}:</span>
                    <span className="font-mono font-bold text-[#1A3A8F]">{item.count}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {/* Sample Rows Table */}
          <div className="bg-white rounded-[8px] border border-[#E2E8F0] shadow-sm overflow-hidden">
            <div className="p-5 border-b border-[#E2E8F0] flex items-center justify-between">
              <div>
                <h3 className="font-bold text-[#0F1729] text-[14px]">
                  Sample Data Rows ({previewData.sample_rows.length} of {previewData.total_records})
                </h3>
                <p className="text-[12px] text-[#64748B]">
                  Inspect extracted rows before inserting into database
                </p>
              </div>
            </div>

            <div className="overflow-x-auto max-h-[440px]">
              <table className="w-full text-left border-collapse text-[12px]">
                <thead className="bg-[#F8FAFC] border-b border-[#E2E8F0] sticky top-0 z-10 text-[11px] uppercase tracking-wider text-[#64748B]">
                  <tr>
                    <th className="py-2.5 px-4 font-semibold">SLNO</th>
                    <th className="py-2.5 px-4 font-semibold">Enrolment No & Date</th>
                    <th className="py-2.5 px-4 font-semibold">Resident Name</th>
                    <th className="py-2.5 px-4 font-semibold">Station ID</th>
                    <th className="py-2.5 px-4 font-semibold">Update Type</th>
                    <th className="py-2.5 px-4 font-semibold text-right">Fee (₹)</th>
                    <th className="py-2.5 px-4 font-semibold">Operator</th>
                    <th className="py-2.5 px-4 font-semibold">District Manager</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#F1F5F9]">
                  {previewData.sample_rows.map((row, idx) => (
                    <tr key={idx} className="hover:bg-slate-50 transition">
                      <td className="py-2 px-4 font-mono text-[#64748B]">{row.slno}</td>
                      <td className="py-2 px-4 font-mono font-medium text-[#0F1729]">{row.enrolment_no_date}</td>
                      <td className="py-2 px-4 text-[#1E293B]">{row.resident_name || '-'}</td>
                      <td className="py-2 px-4 font-mono text-[#1E293B]">{row.station_id}</td>
                      <td className="py-2 px-4">
                        <span className="inline-block px-2 py-0.5 rounded bg-blue-50 text-[#1A3A8F] font-semibold text-[11px]">
                          {row.update_type}
                        </span>
                      </td>
                      <td className="py-2 px-4 text-right font-mono font-medium text-[#0F1729]">
                        ₹{row.total_amount_charged}
                      </td>
                      <td className="py-2 px-4 text-[#475569]">{row.operator_name || row.operator_id}</td>
                      <td className="py-2 px-4 text-[#475569]">{row.district_manager}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

