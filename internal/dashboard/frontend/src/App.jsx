import React, { useState, useEffect, lazy, Suspense } from 'react';
import {
  Squares2X2Icon,
  ChatBubbleLeftRightIcon,
  ClockIcon,
  CircleStackIcon,
  BoltIcon,
  DocumentTextIcon,
  KeyIcon,
  ShieldCheckIcon,
  Bars3Icon,
  XMarkIcon,
  SunIcon,
  MoonIcon,
} from '@heroicons/react/24/outline';
import { ToastProvider } from './components/shared/Toast';
import Spinner from './components/shared/Spinner';
import { initializeTheme, toggleTheme, getTheme } from './utils/theme';

const Dashboard = lazy(() => import('./components/Dashboard'));
const Chat = lazy(() => import('./components/Chat'));
const TaskScheduler = lazy(() => import('./components/TaskScheduler'));
const Memory = lazy(() => import('./components/Memory'));
const Activity = lazy(() => import('./components/Activity'));
const Logs = lazy(() => import('./components/Logs'));
const OAuth = lazy(() => import('./components/OAuth'));
const Audit = lazy(() => import('./components/Audit'));

const TABS = [
  { id: 'dashboard', label: 'Servers', Icon: Squares2X2Icon, Component: Dashboard },
  { id: 'chat', label: 'Chat', Icon: ChatBubbleLeftRightIcon, Component: Chat },
  { id: 'tasks', label: 'Tasks', Icon: ClockIcon, Component: TaskScheduler },
  { id: 'memory', label: 'Memory', Icon: CircleStackIcon, Component: Memory },
  { id: 'activity', label: 'Activity', Icon: BoltIcon, Component: Activity },
  { id: 'logs', label: 'Logs', Icon: DocumentTextIcon, Component: Logs },
  { id: 'oauth', label: 'OAuth', Icon: KeyIcon, Component: OAuth },
  { id: 'audit', label: 'Audit', Icon: ShieldCheckIcon, Component: Audit },
];

function App() {
  const [activeTab, setActiveTab] = useState(() => {
    return localStorage.getItem('activeTab') || 'dashboard';
  });
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [theme, setThemeState] = useState('light');
  const [servers, setServers] = useState([]);

  useEffect(() => {
    const initialTheme = initializeTheme();
    setThemeState(initialTheme);
  }, []);

  useEffect(() => {
    const fetchServers = async () => {
      try {
        const response = await fetch('/api/servers');
        if (response.ok) {
          const data = await response.json();
          const serversArray = Object.keys(data || {}).map(name => ({
            name,
            ...data[name]
          }));
          setServers(serversArray);
        }
      } catch (error) {
        console.error('Failed to fetch servers:', error);
      }
    };

    fetchServers();
    const interval = setInterval(fetchServers, 10000);

    return () => clearInterval(interval);
  }, []);

  const handleTabChange = (tabId) => {
    setActiveTab(tabId);
    localStorage.setItem('activeTab', tabId);
    setMobileMenuOpen(false);
  };

  const handleThemeToggle = () => {
    const newTheme = toggleTheme();
    setThemeState(newTheme);
  };

  const activeTabConfig = TABS.find((tab) => tab.id === activeTab);
  const ActiveComponent = activeTabConfig?.Component;

  return (
    <ToastProvider>
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors duration-200 w-full max-w-full overflow-x-hidden">
        <header className="sticky top-0 z-40 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 shadow-sm w-full">
          <div className="mx-auto px-4 sm:px-6 lg:px-8 w-full max-w-full">
            <div className="flex h-16 items-center justify-between w-full">
              <div className="flex items-center min-w-0 flex-1">
                <button
                  type="button"
                  className="lg:hidden inline-flex items-center justify-center min-h-[44px] min-w-[44px] flex-shrink-0 rounded-md text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                  onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                  aria-label="Toggle menu"
                >
                  {mobileMenuOpen ? (
                    <XMarkIcon className="h-6 w-6" aria-hidden="true" />
                  ) : (
                    <Bars3Icon className="h-6 w-6" aria-hidden="true" />
                  )}
                </button>
                <h1 className="ml-2 lg:ml-0 text-xl font-bold text-gray-900 dark:text-white truncate">
                  MCP Compose
                </h1>
              </div>

              <button
                type="button"
                onClick={handleThemeToggle}
                className="inline-flex items-center justify-center min-h-[44px] min-w-[44px] flex-shrink-0 rounded-lg text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-colors"
                aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
              >
                {theme === 'dark' ? (
                  <SunIcon className="h-5 w-5" aria-hidden="true" />
                ) : (
                  <MoonIcon className="h-5 w-5" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>

          <nav className="hidden lg:block border-t border-gray-200 dark:border-gray-700 w-full">
            <div className="mx-auto px-4 sm:px-6 lg:px-8 w-full max-w-full">
              <div className="flex space-x-1 overflow-x-auto scrollbar-hide">
                {TABS.map((tab) => {
                  const isActive = activeTab === tab.id;

                  return (
                    <button
                      key={tab.id}
                      onClick={() => handleTabChange(tab.id)}
                      className={`
                        inline-flex items-center px-4 py-3 border-b-2 text-sm font-medium min-h-[44px] flex-shrink-0
                        transition-colors whitespace-nowrap focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500
                        ${
                          isActive
                            ? 'border-blue-500 text-blue-600 dark:text-blue-400'
                            : 'border-transparent text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 hover:border-gray-300 dark:hover:border-gray-600'
                        }
                      `}
                      aria-current={isActive ? 'page' : undefined}
                    >
                      <tab.Icon className="h-5 w-5 mr-2 flex-shrink-0" aria-hidden="true" />
                      {tab.label}
                    </button>
                  );
                })}
              </div>
            </div>
          </nav>
        </header>

        {mobileMenuOpen && (
          <div className="lg:hidden fixed inset-0 z-50 bg-black bg-opacity-50" onClick={() => setMobileMenuOpen(false)}>
            <nav className="fixed inset-y-0 left-0 w-64 max-w-[80vw] bg-white dark:bg-gray-800 shadow-xl flex flex-col" onClick={(e) => e.stopPropagation()}>
              <div className="flex-shrink-0 px-4 py-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
                <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Menu</h2>
                <button
                  type="button"
                  onClick={() => setMobileMenuOpen(false)}
                  className="inline-flex items-center justify-center min-h-[44px] min-w-[44px] rounded-md text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500 transition-colors"
                  aria-label="Close menu"
                >
                  <XMarkIcon className="h-6 w-6" aria-hidden="true" />
                </button>
              </div>

              <div className="flex-1 overflow-y-auto px-2 py-4 space-y-1">
                {TABS.map((tab) => {
                  const isActive = activeTab === tab.id;

                  return (
                    <button
                      key={tab.id}
                      onClick={() => handleTabChange(tab.id)}
                      className={`
                        w-full flex items-center px-3 py-3 rounded-md text-base font-medium min-h-[44px]
                        transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500
                        ${
                          isActive
                            ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                        }
                      `}
                      aria-current={isActive ? 'page' : undefined}
                    >
                      <tab.Icon className="h-6 w-6 mr-3 flex-shrink-0" aria-hidden="true" />
                      <span className="truncate">{tab.label}</span>
                    </button>
                  );
                })}
              </div>
            </nav>
          </div>
        )}

        <main className="mx-auto px-4 sm:px-6 lg:px-8 py-6 max-w-7xl w-full overflow-x-hidden">
          <Suspense
            fallback={
              <div className="flex items-center justify-center min-h-[400px]">
                <Spinner size="lg" label={`Loading ${activeTabConfig?.label}...`} />
              </div>
            }
          >
            {ActiveComponent && <ActiveComponent servers={servers} />}
          </Suspense>
        </main>
      </div>
    </ToastProvider>
  );
}

export default App;
