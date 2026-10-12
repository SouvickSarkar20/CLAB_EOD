import React, { useState } from 'react';
import { NavLink } from 'react-router-dom';
import {
  BarChart3,
  Users,
  Calendar,
  Activity,
  
  Download,
  Settings,
  Shield,
  UploadCloud,
  X,
  ChevronRight
} from 'lucide-react';
import { useAuthStore } from '../store/authStore';

export function Sidebar() {
  const user = useAuthStore((s) => s.user);
  const [isOpen, setIsOpen] = useState(true);

  const basePath = user?.role === 'admin' ? '/admin/dashboard' : '/dm/dashboard';

  const navItems = [
    {
      to: basePath,
      label: 'Executive Overview',
      icon: BarChart3,
      adminOnly: false,
      end: true,
    },
    {
      to: `${basePath}/dms`,
      label: 'District Managers',
      icon: Users,
      adminOnly: true,
      end: false,
    },
    {
      to: `${basePath}/monthly`,
      label: 'Monthly Summaries',
      icon: Calendar,
      adminOnly: false,
      end: false,
    },
    {
      to: `${basePath}/daily`,
      label: 'Daily Operational Logs',
      icon: Activity,
      adminOnly: false,
      end: false,
    },
    {
      to: `${basePath}/upload`,
      label: 'Upload Daily EOD',
      icon: UploadCloud,
      adminOnly: false,
      dmOnly: true,
      end: false,
    },
    {
      to: `${basePath}/reports`,
      label: 'Reports & Exports',
      icon: Download,
      adminOnly: false,
      dmOnly: false,
      end: false,
    },
    {
      to: `${basePath}/settings`,
      label: 'Settings & Security',
      icon: Settings,
      adminOnly: false,
      dmOnly: false,
      end: false,
    },
  ];

  return (
    <aside className={`${isOpen ? 'w-64' : 'w-20'} bg-[#0F1729] text-[#9CA3AF] flex flex-col flex-shrink-0 min-h-screen border-r border-[#1E2A45] transition-all duration-300 relative z-20`}>
      {isOpen ? (
        <>
          {/* App Branding */}
          <div className="h-[72px] px-6 border-b border-[#1E2A45] flex items-center justify-between overflow-hidden whitespace-nowrap">
            <div className="flex items-center bg-white px-2 py-1 rounded-md">
              <img src="/logo.png" alt="Computer LAB Logo" className="h-8 object-contain" />
            </div>
            <button 
              onClick={() => setIsOpen(false)} 
              className="text-[#9CA3AF] hover:text-white p-1 rounded flex-shrink-0 ml-1 transition"
              title="Close Sidebar"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* Navigation List */}
          <nav className="flex-1 py-4 space-y-1 overflow-y-auto overflow-x-hidden">
            {navItems.map((item) => {
              if (item.adminOnly && user?.role !== 'admin') {
                return null;
              }
              if ((item as any).dmOnly && user?.role === 'admin') {
                return null;
              }

              const Icon = item.icon;

              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  className={({ isActive }) =>
                    `flex items-center space-x-3.5 px-6 py-2.5 text-[13px] font-semibold transition whitespace-nowrap overflow-hidden border-l-[3px] ${
                      isActive
                        ? 'bg-[#1A3A8F] text-white border-[#1A3A8F]'
                        : 'border-transparent text-[#9CA3AF] hover:bg-[#1E2A45] hover:text-white'
                    }`
                  }
                >
                  <Icon className="w-4 h-4 flex-shrink-0" />
                  <span>{item.label}</span>
                </NavLink>
              );
            })}
          </nav>

          {/* User Status Card at Sidebar Bottom */}
          <div className="px-6 py-4 border-t border-[#1E2A45] bg-[#0F1729] whitespace-nowrap overflow-hidden">
            <div className="flex items-center space-x-3">
              <div className="w-8 h-8 rounded bg-[#1A3A8F] text-white font-bold flex flex-shrink-0 items-center justify-center text-[13px]">
                {user?.name?.charAt(0) || 'U'}
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-[13px] font-bold text-white truncate">{user?.name}</p>
                <p className="text-[11px] text-[#9CA3AF] truncate capitalize">{user?.role}</p>
              </div>
            </div>
          </div>
        </>
      ) : (
        <div className="flex flex-col items-center py-6 h-full">
          <button 
            onClick={() => setIsOpen(true)} 
            className="p-2 rounded hover:bg-[#1E2A45] text-white transition mb-6"
            title="Open Sidebar"
          >
            <ChevronRight className="w-5 h-5" />
          </button>
          
          <nav className="flex-1 space-y-2 flex flex-col w-full overflow-y-auto overflow-x-hidden">
            {navItems.map((item) => {
              if (item.adminOnly && user?.role !== 'admin') return null;
              const Icon = item.icon;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.end}
                  title={item.label}
                  className={({ isActive }) =>
                    `flex justify-center py-3 transition border-l-[3px] ${
                      isActive
                        ? 'bg-[#1A3A8F] text-white border-[#1A3A8F]'
                        : 'border-transparent text-[#9CA3AF] hover:bg-[#1E2A45] hover:text-white'
                    }`
                  }
                >
                  <Icon className="w-5 h-5 flex-shrink-0" />
                </NavLink>
              );
            })}
          </nav>
        </div>
      )}
    </aside>
  );
}
