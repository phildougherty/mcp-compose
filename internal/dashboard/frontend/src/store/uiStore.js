import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';

/**
 * @typedef {'light' | 'dark' | 'system'} Theme
 */

/**
 * @typedef {Object} ToastNotification
 * @property {string} id - Unique toast identifier
 * @property {string} message - Toast message
 * @property {'success' | 'error' | 'warning' | 'info'} type - Toast type
 * @property {number} [duration] - Duration in ms (default: 5000)
 * @property {number} timestamp - Creation timestamp
 */

/**
 * @typedef {Object} Modal
 * @property {string} id - Modal identifier
 * @property {string} type - Modal type/component name
 * @property {Object} [data] - Modal data/props
 * @property {boolean} dismissible - Can be dismissed by clicking outside
 */

/**
 * @typedef {Object} UIState
 * @property {Theme} theme - Current theme
 * @property {boolean} isMobileMenuOpen - Mobile menu open state
 * @property {boolean} isSidebarCollapsed - Sidebar collapsed state (desktop)
 * @property {boolean} isChatSidebarOpen - Chat sidebar open state (mobile)
 * @property {string} activeTab - Currently active tab
 * @property {ToastNotification[]} toasts - Active toast notifications
 * @property {Modal[]} modals - Active modals
 * @property {boolean} isOnline - Online/offline status
 * @property {Object<string, any>} preferences - User preferences
 */

/**
 * @typedef {Object} UIActions
 * @property {(theme: Theme) => void} setTheme - Set theme
 * @property {() => void} toggleTheme - Toggle between light and dark
 * @property {(open: boolean) => void} setMobileMenuOpen - Set mobile menu state
 * @property {() => void} toggleMobileMenu - Toggle mobile menu
 * @property {(collapsed: boolean) => void} setSidebarCollapsed - Set sidebar collapsed state
 * @property {() => void} toggleSidebar - Toggle sidebar
 * @property {(open: boolean) => void} setChatSidebarOpen - Set chat sidebar state
 * @property {() => void} toggleChatSidebar - Toggle chat sidebar
 * @property {(tab: string) => void} setActiveTab - Set active tab
 * @property {(message: string, type: ToastNotification['type'], duration?: number) => string} showToast - Show toast notification
 * @property {(id: string) => void} hideToast - Hide toast notification
 * @property {() => void} clearToasts - Clear all toasts
 * @property {(type: string, data?: Object, dismissible?: boolean) => string} openModal - Open modal
 * @property {(id: string) => void} closeModal - Close modal
 * @property {() => void} closeAllModals - Close all modals
 * @property {(online: boolean) => void} setOnline - Set online status
 * @property {(key: string, value: any) => void} setPreference - Set user preference
 * @property {() => void} reset - Reset UI state (except persisted theme)
 */

/**
 * @typedef {UIState & UIActions} UIStore
 */

const initialState = {
  theme: 'system',
  isMobileMenuOpen: false,
  isSidebarCollapsed: false,
  isChatSidebarOpen: false,
  activeTab: 'dashboard',
  toasts: [],
  modals: [],
  isOnline: typeof navigator !== 'undefined' ? navigator.onLine : true,
  preferences: {
    animations: true,
    autoScroll: true,
    showTimestamps: true,
    lineWrap: true,
    compactMode: false,
  },
};

/**
 * Determine the actual theme to apply based on theme setting and system preference
 * @param {Theme} theme - Theme setting
 * @returns {'light' | 'dark'} Resolved theme
 */
const resolveTheme = (theme) => {
  if (theme === 'system') {
    if (typeof window !== 'undefined') {
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    return 'light';
  }

  return theme;
};

/**
 * Apply theme to document
 * @param {Theme} theme - Theme to apply
 */
const applyTheme = (theme) => {
  if (typeof document === 'undefined') return;

  const resolved = resolveTheme(theme);
  const root = document.documentElement;

  if (resolved === 'dark') {
    root.classList.add('dark');
  } else {
    root.classList.remove('dark');
  }
};

/**
 * UI Store - Manages UI state, theme, modals, and notifications
 *
 * @description
 * Centralized state management for UI concerns including theme,
 * mobile menu, sidebar, modals, toast notifications, and user preferences.
 * Theme preference is persisted to localStorage.
 * Integrates with DevTools for debugging.
 *
 * @example
 * // Toggle theme
 * const toggleTheme = useUIStore(state => state.toggleTheme);
 * toggleTheme();
 *
 * @example
 * // Show toast notification
 * const showToast = useUIStore(state => state.showToast);
 * showToast('Server started successfully', 'success');
 *
 * @example
 * // Open modal
 * const openModal = useUIStore(state => state.openModal);
 * openModal('confirm-delete', { serverId: 'server-1' });
 */
export const useUIStore = create(
  devtools(
    persist(
      (set, get) => ({
        ...initialState,

        setTheme: (theme) => {
          applyTheme(theme);
          set({ theme }, false, 'setTheme');
        },

        toggleTheme: () => {
          const currentTheme = get().theme;
          let newTheme;

          if (currentTheme === 'system') {
            const resolved = resolveTheme('system');
            newTheme = resolved === 'light' ? 'dark' : 'light';
          } else {
            newTheme = currentTheme === 'light' ? 'dark' : 'light';
          }

          applyTheme(newTheme);
          set({ theme: newTheme }, false, 'toggleTheme');
        },

        setMobileMenuOpen: (isMobileMenuOpen) => set({
          isMobileMenuOpen
        }, false, 'setMobileMenuOpen'),

        toggleMobileMenu: () => set((state) => ({
          isMobileMenuOpen: !state.isMobileMenuOpen
        }), false, 'toggleMobileMenu'),

        setSidebarCollapsed: (isSidebarCollapsed) => set({
          isSidebarCollapsed
        }, false, 'setSidebarCollapsed'),

        toggleSidebar: () => set((state) => ({
          isSidebarCollapsed: !state.isSidebarCollapsed
        }), false, 'toggleSidebar'),

        setChatSidebarOpen: (isChatSidebarOpen) => set({
          isChatSidebarOpen
        }, false, 'setChatSidebarOpen'),

        toggleChatSidebar: () => set((state) => ({
          isChatSidebarOpen: !state.isChatSidebarOpen
        }), false, 'toggleChatSidebar'),

        setActiveTab: (activeTab) => set({ activeTab }, false, 'setActiveTab'),

        showToast: (message, type, duration = 5000) => {
          const id = `toast-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
          const toast = {
            id,
            message,
            type,
            duration,
            timestamp: Date.now(),
          };

          set((state) => ({
            toasts: [...state.toasts, toast],
          }), false, 'showToast');

          if (duration > 0) {
            setTimeout(() => {
              get().hideToast(id);
            }, duration);
          }

          return id;
        },

        hideToast: (id) => set((state) => ({
          toasts: state.toasts.filter(t => t.id !== id),
        }), false, 'hideToast'),

        clearToasts: () => set({ toasts: [] }, false, 'clearToasts'),

        openModal: (type, data, dismissible = true) => {
          const id = `modal-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
          const modal = { id, type, data, dismissible };

          set((state) => ({
            modals: [...state.modals, modal],
          }), false, 'openModal');

          return id;
        },

        closeModal: (id) => set((state) => ({
          modals: state.modals.filter(m => m.id !== id),
        }), false, 'closeModal'),

        closeAllModals: () => set({ modals: [] }, false, 'closeAllModals'),

        setOnline: (isOnline) => set({ isOnline }, false, 'setOnline'),

        setPreference: (key, value) => set((state) => ({
          preferences: { ...state.preferences, [key]: value },
        }), false, 'setPreference'),

        reset: () => set((state) => ({
          ...initialState,
          theme: state.theme,
        }), false, 'reset'),
      }),
      {
        name: 'ui-store',
        partialState: (state) => ({
          theme: state.theme,
          preferences: state.preferences,
        }),
      }
    ),
    {
      name: 'ui-store',
      enabled: process.env.NODE_ENV === 'development',
    }
  )
);

/**
 * Optimized selectors to prevent unnecessary re-renders
 */

export const selectTheme = (state) => state.theme;

export const selectResolvedTheme = (state) => resolveTheme(state.theme);

export const selectIsMobileMenuOpen = (state) => state.isMobileMenuOpen;

export const selectIsSidebarCollapsed = (state) => state.isSidebarCollapsed;

export const selectIsChatSidebarOpen = (state) => state.isChatSidebarOpen;

export const selectActiveTab = (state) => state.activeTab;

export const selectToasts = (state) => state.toasts;

export const selectModals = (state) => state.modals;

export const selectIsOnline = (state) => state.isOnline;

export const selectPreferences = (state) => state.preferences;

export const selectPreference = (key) => (state) => state.preferences[key];

/**
 * Initialize theme on app load
 * Call this once when the app starts
 */
export const initializeTheme = () => {
  const theme = useUIStore.getState().theme;
  applyTheme(theme);

  if (typeof window !== 'undefined') {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    mediaQuery.addEventListener('change', () => {
      const currentTheme = useUIStore.getState().theme;
      if (currentTheme === 'system') {
        applyTheme('system');
      }
    });

    window.addEventListener('online', () => {
      useUIStore.getState().setOnline(true);
    });

    window.addEventListener('offline', () => {
      useUIStore.getState().setOnline(false);
    });
  }
};
