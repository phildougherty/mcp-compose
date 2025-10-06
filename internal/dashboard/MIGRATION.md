# Dashboard Migration Plan: Vue.js → React.js (Mobile-First)

## Executive Summary

This document outlines a comprehensive migration plan for transitioning the MCP Compose Dashboard from Vue 3 (CDN-based) to a modern, **mobile-first React.js** implementation using Tailwind CSS and Headless UI. The migration is organized into parallelizable tasks to enable multiple agents to work simultaneously without conflicts.

**Key Metrics:**
- **Total Vue.js Components:** 11 components (~11,160 lines of JS code)
- **Backend Integration:** Go-based REST API + WebSocket + PostgreSQL
- **Frontend Stack (Current):** Vue 3 CDN, Tailwind CSS CDN, vanilla JavaScript
- **Target Stack:** React.js 18+, Tailwind CSS 3+, Headless UI, TypeScript (optional)

---

## Table of Contents

1. [Current Architecture](#1-current-architecture)
2. [All Existing Features](#2-all-existing-features)
3. [Complete Component Inventory](#3-complete-component-inventory)
4. [Migration Strategy](#4-migration-strategy)
5. [Mobile-First Technical Requirements](#5-mobile-first-technical-requirements)
6. [Parallel Task Breakdown](#6-parallel-task-breakdown)
7. [Component Dependencies](#7-component-dependencies)
8. [Acceptance Criteria](#8-acceptance-criteria)

---

## 1. Current Architecture

### 1.1 Backend Architecture

**Language:** Go 1.19+
**Framework:** Chi router
**Database:** PostgreSQL
**Real-time:** WebSocket connections for live updates

**Backend Files:**
- `/home/phil/dev/mcp-compose/internal/dashboard/manager.go` - Server/scheduler interfaces
- `/home/phil/dev/mcp-compose/internal/dashboard/server.go` - HTTP server setup
- `/home/phil/dev/mcp-compose/internal/dashboard/handlers.go` - REST API handlers
- `/home/phil/dev/mcp-compose/internal/dashboard/websocket.go` - WebSocket connections
- `/home/phil/dev/mcp-compose/internal/dashboard/inspector_handlers.go` - MCP inspector API
- `/home/phil/dev/mcp-compose/internal/dashboard/inspector_service.go` - Inspector business logic
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_handlers.go` - Chat API endpoints
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go` - Chat service layer
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_storage.go` - PostgreSQL chat storage
- `/home/phil/dev/mcp-compose/internal/dashboard/activity_storage.go` - PostgreSQL activity storage
- `/home/phil/dev/mcp-compose/internal/dashboard/system_tools.go` - System tools manager

**Database Tables:**
- `chat_sessions` - Chat session metadata
- `chat_messages` - Chat message history
- `activity_events` - Activity log storage
- OAuth tables (managed by OAuth server)
- Audit log tables (managed by audit system)

### 1.2 Frontend Architecture (Current)

**Framework:** Vue 3 (global production build from CDN)
**Styling:** Tailwind CSS 3 (from CDN)
**State Management:** Component-level reactive data
**HTTP Client:** Native `fetch` API
**WebSocket:** Native WebSocket API

**Frontend Files:**
```
internal/dashboard/templates/
├── index.html                           # Main HTML template (78 lines)
└── static/
    ├── app.js                           # Vue app initialization
    ├── utils.js                         # Global utility functions
    ├── style.css                        # Global styles (2,234 lines)
    └── components/
        ├── dashboard.js                 # Main dashboard (2,718 lines)
        ├── task-scheduler.js            # Task scheduler (3,136 lines)
        ├── memory.js                    # Memory viewer (2,638 lines)
        ├── oauth.js                     # OAuth config (1,499 lines)
        ├── chat.js                      # Chat interface (1,021 lines)
        ├── audit.js                     # Audit logs (1,165 lines)
        ├── server-oauth.js              # Server OAuth (1,022 lines)
        ├── activity.js                  # Activity viewer (856 lines)
        ├── logs.js                      # Log viewer (783 lines)
        ├── inspector.js                 # MCP Inspector (455 lines)
        └── [other component files]
```

### 1.3 Key Technologies & Patterns

**Current Patterns:**
- Component-based architecture with Vue 3 Options API
- Props for parent-child communication
- Custom events for child-parent communication (`$emit`)
- Computed properties for derived state
- Watchers for reactive side effects
- Lifecycle hooks (`mounted`, `beforeUnmount`)
- Manual DOM refs (`$refs`)
- CDN-based dependencies (no build step)

**API Communication:**
- REST API: `/api/*` endpoints
- WebSocket: `ws://host/ws/*` endpoints
- Authentication: Bearer token in Authorization header
- Real-time: WebSocket subscriptions for live data

---

## 2. All Existing Features

### 2.1 Core Dashboard Features

#### Server Management (dashboard.js)
- **Server Overview:** Display all MCP servers with status cards
- **Server Metrics:** Total servers, running count, healthy count, active connections, proxy uptime
- **Server Actions:** Start, stop, restart individual servers
- **Real-time Status:** Live updates of server health and connection state
- **Server Details:** Expand/collapse server info with capabilities, tools, health
- **Search & Filter:** Filter by name, status (running/stopped/healthy)
- **Sort Options:** By name, status, health, tool count
- **Auto-refresh:** Configurable auto-refresh with intervals (5s, 10s, 30s, 1m, 5m)
- **Proxy Management:** Restart proxy with confirmation dialog
- **Mobile-responsive:** Hamburger menu, collapsible stats

#### Chat Interface (chat.js)
- **Multi-session Chat:** Create, load, and manage multiple chat sessions
- **AI Provider Support:** OpenAI, Anthropic, Ollama, OpenRouter
- **Model Selection:** Dropdown for different AI models per provider
- **MCP Server Integration:** Select which MCP servers to connect to chat
- **Tool Execution Visualization:** Live display of tool calls during AI responses
- **Streaming Responses:** Real-time streaming of AI responses via WebSocket
- **Message History:** Persistent chat history with PostgreSQL storage
- **Session Management:** Create, rename, delete chat sessions
- **System Prompt Viewer:** Inspect system prompts sent to AI
- **Markdown Rendering:** Rich text formatting in messages
- **Code Block Support:** Syntax highlighting for code snippets
- **Mobile Sidebar:** Collapsible session list with hamburger menu
- **Connection Status:** Live WebSocket connection indicator

#### MCP Inspector (inspector.js)
- **Server Connection:** Connect to any MCP server for inspection
- **Protocol Testing:** Send MCP JSON-RPC 2.0 requests
- **Request Templates:** Pre-built templates for common MCP methods
  - `initialize` - Server initialization
  - `tools/list` - List available tools
  - `resources/list` - List available resources
  - `prompts/list` - List available prompts
  - `tools/call` - Execute a tool
- **Response Viewer:** JSON-formatted response display
- **Tool Discovery:** Automatic tool discovery and cataloging
- **Availability Check:** Gracefully handle missing inspector endpoints
- **Auto-connect:** Connect to available servers on mount

#### Task Scheduler (task-scheduler.js)
- **Task Types:**
  - Shell commands (cron scheduled)
  - AI-powered tasks (LLM execution)
  - Manual tasks (triggered on demand)
  - Dependency tasks (chained execution)
  - Watcher tasks (file/event monitoring)
- **Cron Scheduling:** Full cron expression support with presets
- **Task Management:** Create, edit, delete, enable/disable tasks
- **Task Execution:** Manual trigger, view output, view logs
- **Task History:** Recent runs with status and duration
- **Task Statistics:** Total tasks, enabled, running, completed/failed (24h)
- **Task Groups:** Organize tasks by type with accordion UI
- **Search & Filter:** By name, description, command, prompt
- **Sort Options:** By name, type, status, schedule, last run
- **Auto-refresh:** Periodic task status updates
- **AI Task Configuration:** Model hints, max cost, local-only options

#### Memory Management (memory.js)
- **Entity CRUD:** Create, read, update, delete entities
- **Entity Types:** Custom entity type classification
- **Observations:** Add/delete observations per entity
- **Relationships:** Create/delete relationships between entities
- **Graph Visualization:** Visual knowledge graph (planned)
- **Search:** Full-text search across entities and observations
- **Filtering:** By entity type, date range
- **Pagination:** Paginated entity list (50 per page)
- **Bulk Operations:** Select multiple entities for batch delete
- **Statistics:** Entity count, relation count, type distribution

#### Activity Monitor (activity.js)
- **Real-time Stream:** Live activity events via WebSocket
- **Historical Data:** Load 6 hours of historical events from PostgreSQL
- **Event Types:**
  - Requests (API calls)
  - Connections (WebSocket connections)
  - Tool calls (MCP tool executions)
  - Errors (system errors)
- **Event Levels:** ERROR, WARN, INFO, DEBUG
- **Statistics:** Total events, requests, tool calls, errors
- **Filtering:** By level, type, search term
- **Tool Call Details:** Expand tool calls to view parameters and results
- **Auto-scroll:** Automatic scroll to new events

#### Log Viewer (logs.js)
- **Container Logs:** View Docker/Podman container logs
- **Server Selection:** Dropdown to select which server's logs to view
- **Log Streaming:** Real-time log streaming via WebSocket
- **Log Levels:** Auto-detection of ERROR, WARN, INFO, DEBUG
- **Log Actions:**
  - Load last 100 lines
  - Start/stop streaming
  - Clear logs
  - Download logs as .txt file
- **Search:** Filter logs by search term
- **Level Filter:** Filter by log level
- **Display Options:** Show timestamps, auto-scroll, line wrap
- **Syntax Highlighting:** Color-coded log levels
- **Terminal UI:** macOS-style terminal window with traffic lights

#### OAuth Configuration (oauth.js)
- **OAuth Server Status:** Display OAuth enabled/disabled state
- **Token Statistics:** Active access tokens, refresh tokens, auth codes
- **Client Management:**
  - Register new OAuth clients
  - View client details (ID, secret, redirect URIs)
  - Delete OAuth clients
  - Public vs. Confidential clients
- **OAuth Endpoints:** Display authorization, token, and discovery endpoints
- **Test Flows:**
  - Authorization code flow testing
  - Client credentials flow testing
- **Search & Filter:** By client name, type (public/confidential)
- **Client Statistics:** Total clients, public count, confidential count

#### Audit Logs (audit.js)
- **Audit Events:** Track security and system events
- **Event Types:**
  - Token issued/revoked
  - User login/logout
  - Access granted/denied
  - Client created/deleted
  - Config changes
- **Statistics:** Total events, success rate, success/failure counts
- **Event Distribution:** Chart of events by type
- **Filtering:** By event type, success/failure, time range (1h, 24h, 7d, 30d, all)
- **Search:** Full-text search across audit entries
- **Pagination:** Page through audit entries
- **Export:** Export audit logs to CSV
- **Auto-refresh:** Periodic audit log updates
- **Event Details:** Expand rows to view full event metadata

### 2.2 Shared UI Components & Patterns

#### Navigation
- **Tab Navigation:** Multi-tab interface with icon + label
- **Mobile Menu:** Hamburger menu for mobile devices
- **Breadcrumbs:** Navigation context (where applicable)

#### Forms
- **Text Inputs:** Standard text input with validation
- **Textareas:** Multi-line text input
- **Select Dropdowns:** Single and multi-select
- **Checkboxes:** Boolean toggles
- **Radio Buttons:** Single choice from multiple options
- **File Uploads:** File selection (where applicable)

#### Data Display
- **Tables:** Paginated data tables with sort
- **Cards:** Card-based layouts for entities
- **Lists:** List views for items
- **Accordions:** Collapsible sections
- **Modals:** Popup dialogs for details/forms
- **Toasts:** Toast notifications for success/error/warning/info

#### Loading States
- **Spinners:** Loading indicators
- **Skeletons:** Skeleton loading placeholders
- **Progress Bars:** Progress indicators

#### Empty States
- **No Data:** Friendly empty state messages
- **No Results:** Search/filter empty states
- **Error States:** Error messages with retry actions

---

## 3. Complete Component Inventory

### Vue Components to Migrate (11 Total)

| # | Component | File | Lines | Description | Dependencies |
|---|-----------|------|-------|-------------|--------------|
| 1 | `DashboardApp` | dashboard.js | 2,718 | Main dashboard, server overview | All child components |
| 2 | `TaskScheduler` | task-scheduler.js | 3,136 | Task management and scheduling | None (standalone) |
| 3 | `MemoryViewer` | memory.js | 2,638 | Knowledge graph management | None (standalone) |
| 4 | `OAuthConfig` | oauth.js | 1,499 | OAuth client configuration | None (standalone) |
| 5 | `ChatComponent` | chat.js | 1,021 | AI chat interface | None (standalone) |
| 6 | `AuditLog` | audit.js | 1,165 | Audit log viewer | None (standalone) |
| 7 | `ServerOAuthConfig` | server-oauth.js | 1,022 | Per-server OAuth config | None (standalone) |
| 8 | `ActivityViewer` | activity.js | 856 | Activity monitor | None (standalone) |
| 9 | `LogViewer` | logs.js | 783 | Container log viewer | None (standalone) |
| 10 | `MCPInspector` | inspector.js | 455 | MCP protocol inspector | None (standalone) |
| 11 | Other components | [various] | ~527 | Smaller components | Various |

**Total:** ~11,160 lines of Vue.js component code

### Shared Utilities (utils.js)

**Functions to migrate:**
- `showToast()` - Toast notification system
- `isMobile()`, `isTablet()`, `isDesktop()` - Responsive detection
- `formatBytes()`, `formatDuration()` - Formatting utilities
- `debounce()` - Debounce utility
- `copyToClipboard()` - Clipboard operations
- Theme management utilities

---

## 4. Migration Strategy

### 4.1 Build System Setup

**New Build System:**
- **Bundler:** Vite (fast, modern)
- **Package Manager:** npm or pnpm
- **Dev Server:** Vite dev server with HMR
- **Production Build:** Optimized production bundles

**Dependencies to add:**
```json
{
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "@headlessui/react": "^1.7.0",
    "@heroicons/react": "^2.0.0",
    "tailwindcss": "^3.3.0",
    "autoprefixer": "^10.4.0",
    "postcss": "^8.4.0",
    "clsx": "^2.0.0",
    "zustand": "^4.4.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.0.0",
    "vite": "^4.4.0",
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "typescript": "^5.0.0"
  }
}
```

### 4.2 Directory Structure

**New React Structure:**
```
internal/dashboard/frontend/
├── package.json
├── vite.config.js
├── tailwind.config.js
├── postcss.config.js
├── index.html
├── src/
│   ├── main.jsx                      # App entry point
│   ├── App.jsx                       # Root App component
│   ├── components/
│   │   ├── Dashboard/                # Main dashboard
│   │   │   ├── Dashboard.jsx
│   │   │   ├── ServerCard.jsx
│   │   │   ├── ServerMetrics.jsx
│   │   │   └── index.js
│   │   ├── Chat/                     # Chat interface
│   │   │   ├── Chat.jsx
│   │   │   ├── MessageList.jsx
│   │   │   ├── ChatInput.jsx
│   │   │   ├── SessionList.jsx
│   │   │   └── index.js
│   │   ├── TaskScheduler/            # Task scheduler
│   │   │   ├── TaskScheduler.jsx
│   │   │   ├── TaskList.jsx
│   │   │   ├── TaskCard.jsx
│   │   │   ├── TaskForm.jsx
│   │   │   └── index.js
│   │   ├── Memory/                   # Memory viewer
│   │   ├── Inspector/                # MCP Inspector
│   │   ├── Activity/                 # Activity monitor
│   │   ├── Logs/                     # Log viewer
│   │   ├── OAuth/                    # OAuth config
│   │   ├── Audit/                    # Audit logs
│   │   └── shared/                   # Shared components
│   │       ├── Button.jsx
│   │       ├── Input.jsx
│   │       ├── Modal.jsx
│   │       ├── Toast.jsx
│   │       ├── Card.jsx
│   │       ├── Badge.jsx
│   │       ├── Spinner.jsx
│   │       └── EmptyState.jsx
│   ├── hooks/                        # Custom React hooks
│   │   ├── useWebSocket.js
│   │   ├── useApi.js
│   │   ├── useToast.js
│   │   ├── usePagination.js
│   │   └── useResponsive.js
│   ├── store/                        # Zustand state management
│   │   ├── dashboardStore.js
│   │   ├── chatStore.js
│   │   └── uiStore.js
│   ├── api/                          # API client layer
│   │   ├── client.js
│   │   ├── dashboard.js
│   │   ├── chat.js
│   │   ├── tasks.js
│   │   ├── memory.js
│   │   └── websocket.js
│   ├── utils/                        # Utility functions
│   │   ├── format.js
│   │   ├── validation.js
│   │   ├── responsive.js
│   │   └── clipboard.js
│   └── styles/                       # Global styles
│       ├── index.css
│       └── tailwind.css
└── public/                           # Static assets
```

### 4.3 Migration Phases

**Phase 1: Foundation (Week 1)**
- Set up build system (Vite + React)
- Configure Tailwind CSS
- Create base layout structure
- Implement shared UI components
- Set up API client layer
- Implement custom hooks

**Phase 2: Core Components (Week 2-3)**
- Migrate Dashboard component
- Migrate Chat component
- Migrate Inspector component
- Migrate Activity viewer
- Migrate Log viewer

**Phase 3: Advanced Components (Week 3-4)**
- Migrate Task Scheduler
- Migrate Memory viewer
- Migrate OAuth configuration
- Migrate Audit logs

**Phase 4: Polish & Testing (Week 4-5)**
- Mobile responsiveness testing
- Cross-browser testing
- Performance optimization
- Accessibility audit (WCAG 2.1 AA)
- User acceptance testing

---

## 5. Mobile-First Technical Requirements

### 5.1 Responsive Breakpoints

**Tailwind CSS Breakpoints:**
```javascript
// tailwind.config.js
module.exports = {
  theme: {
    screens: {
      'xs': '320px',    // Extra small phones
      'sm': '640px',    // Small phones
      'md': '768px',    // Tablets
      'lg': '1024px',   // Laptops
      'xl': '1280px',   // Desktops
      '2xl': '1536px',  // Large desktops
    }
  }
}
```

### 5.2 Touch Target Sizes

**Minimum Touch Targets:**
- All interactive elements: **44×44px minimum** (WCAG 2.1 AA)
- Spacing between touch targets: **8px minimum**
- Primary action buttons: **48×48px or larger**

**Implementation:**
```jsx
// Button component with proper touch target
<button className="min-h-[44px] min-w-[44px] p-3 ...">
  {children}
</button>
```

### 5.3 Mobile Navigation

**Navigation Patterns:**
- **Hamburger Menu:** For top-level navigation on mobile
- **Bottom Tab Bar:** For primary sections (optional)
- **Swipe Gestures:** For navigating between tabs
- **Pull-to-Refresh:** For refreshing data

**Implementation:**
```jsx
import { Dialog, Transition } from '@headlessui/react'

function MobileMenu({ isOpen, onClose }) {
  return (
    <Transition show={isOpen}>
      <Dialog onClose={onClose} className="relative z-50">
        {/* Mobile menu content */}
      </Dialog>
    </Transition>
  )
}
```

### 5.4 Mobile Forms

**Form Requirements:**
- **Large input fields:** Minimum 48px height
- **Font size:** Minimum 16px to prevent zoom on iOS
- **Input types:** Use semantic HTML5 input types
- **Labels:** Always visible, not placeholders only
- **Validation:** Inline validation with clear error messages

**Example:**
```jsx
<label className="block text-sm font-medium mb-2">
  Email
</label>
<input
  type="email"
  className="w-full h-12 px-4 text-base rounded-lg border ..."
  placeholder="you@example.com"
/>
```

### 5.5 Performance Requirements

**Core Web Vitals:**
- **First Contentful Paint (FCP):** < 1.8s
- **Largest Contentful Paint (LCP):** < 2.5s
- **First Input Delay (FID):** < 100ms
- **Cumulative Layout Shift (CLS):** < 0.1

**Bundle Size:**
- Initial JS bundle: < 200KB gzipped
- CSS bundle: < 50KB gzipped
- Code splitting: Lazy load routes and heavy components

**Implementation:**
```jsx
import { lazy, Suspense } from 'react'

const TaskScheduler = lazy(() => import('./components/TaskScheduler'))

function App() {
  return (
    <Suspense fallback={<Spinner />}>
      <TaskScheduler />
    </Suspense>
  )
}
```

### 5.6 Accessibility Requirements

**WCAG 2.1 Level AA:**
- **Color contrast:** Minimum 4.5:1 for text, 3:1 for large text
- **Keyboard navigation:** All interactive elements keyboard accessible
- **Focus indicators:** Visible focus states on all interactive elements
- **ARIA labels:** Proper ARIA attributes for screen readers
- **Semantic HTML:** Use semantic HTML5 elements

**Implementation:**
```jsx
<button
  aria-label="Close dialog"
  className="focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ..."
>
  <XIcon className="w-5 h-5" />
</button>
```

### 5.7 Dark Mode Support

**Theme Implementation:**
- Use Tailwind's `dark:` variant
- Persist theme preference in localStorage
- System preference detection
- Smooth theme transitions

**Example:**
```jsx
<div className="bg-white dark:bg-gray-900 text-gray-900 dark:text-white">
  {/* Content */}
</div>
```

---

## 6. Parallel Task Breakdown

### Task Group A: Foundation & Infrastructure (No Dependencies)

#### **✅ TASK A1: Build System & Tooling Setup** [COMPLETED]
**Agent:** Build System Specialist
**Estimated Time:** 4 hours
**Files Created:**
- ✅ `internal/dashboard/frontend/package.json`
- ✅ `internal/dashboard/frontend/vite.config.js`
- ✅ `internal/dashboard/frontend/tailwind.config.js`
- ✅ `internal/dashboard/frontend/postcss.config.js`
- ✅ `internal/dashboard/frontend/index.html`
- ✅ `internal/dashboard/frontend/.gitignore`

**Acceptance Criteria:**
- ✅ Vite dev server configured on port 3000
- ✅ Tailwind CSS properly configured with mobile-first breakpoints
- ✅ Hot Module Replacement (HMR) enabled
- ✅ Production build configured with optimization (< 200KB gzipped JS target)
- ✅ Code splitting configured for vendor chunks

---

#### **✅ TASK A2: Shared UI Components Library** [COMPLETED]
**Agent:** UI Components Specialist
**Estimated Time:** 8 hours
**Files Created:**
- ✅ `src/components/shared/Button.jsx` - Multi-variant button with loading state, 44x44px touch targets
- ✅ `src/components/shared/Input.jsx` - Text input with label, error, hint, icons, 48px height, 16px font
- ✅ `src/components/shared/Select.jsx` - Headless UI Listbox dropdown with search
- ✅ `src/components/shared/Checkbox.jsx` - Headless UI Switch variant + standard checkbox
- ✅ `src/components/shared/Modal.jsx` - Headless UI Dialog with transitions
- ✅ `src/components/shared/Toast.jsx` - ToastProvider, useToast hook, notification system
- ✅ `src/components/shared/Card.jsx` - Container with variants, header, footer
- ✅ `src/components/shared/Badge.jsx` - Status indicators with dot variant
- ✅ `src/components/shared/Spinner.jsx` - Loading indicator with fullScreen option
- ✅ `src/components/shared/EmptyState.jsx` - No data states with icon, title, description, action
- ✅ `src/components/shared/Pagination.jsx` - Page navigation with ellipsis, first/last buttons
- ✅ `src/components/shared/SearchInput.jsx` - Search field with icon, clear button, debounce
- ✅ `src/components/shared/index.js` - Centralized exports

**Acceptance Criteria:**
- ✅ All components use Headless UI where applicable (Dialog, Listbox, Switch)
- ✅ All components are fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All interactive elements meet 44×44px touch target minimum (min-h/min-w-[44px])
- ✅ All components have dark mode support (dark: variant throughout)
- ✅ All components use clsx for conditional classes
- ✅ Proper ARIA labels and keyboard navigation implemented

---

#### **✅ TASK A3: API Client Layer** [COMPLETED]
**Agent:** API Specialist
**Estimated Time:** 6 hours
**Files Created:**
- ✅ `src/api/client.js` - Base API client with fetch wrapper, Authorization header, error handling, request/response interceptors
- ✅ `src/api/dashboard.js` - Dashboard API methods (servers, status, metrics, actions: start/stop/restart)
- ✅ `src/api/chat.js` - Chat API methods (sessions, messages, streaming via WebSocket)
- ✅ `src/api/tasks.js` - Task scheduler API methods (task CRUD, execution, history)
- ✅ `src/api/memory.js` - Memory API methods (entities, observations, relationships)
- ✅ `src/api/activity.js` - Activity API methods (activity events, historical data)
- ✅ `src/api/oauth.js` - OAuth API methods (clients, tokens, endpoints)
- ✅ `src/api/audit.js` - Audit API methods (audit logs, export)
- ✅ `src/api/inspector.js` - MCP Inspector API methods (connect, request, disconnect, templates)
- ✅ `src/api/websocket.js` - WebSocket connection manager with auto-reconnection logic
- ✅ `src/api/index.js` - Centralized exports for all API modules

**Acceptance Criteria:**
- ✅ All API methods return typed responses (JSDoc comments added)
- ✅ Error handling is consistent across all methods (centralized error handler)
- ✅ Authorization header is automatically added from localStorage
- ✅ Request/response interceptors work correctly (interceptor arrays implemented)
- ✅ WebSocket reconnection logic is implemented (exponential backoff, max attempts, heartbeat)

---

#### **✅ TASK A4: Custom React Hooks** [COMPLETED]
**Agent:** Hooks Specialist
**Estimated Time:** 6 hours
**Files Created:**
- ✅ `src/hooks/useWebSocket.js` - WebSocket connection hook with reconnection logic
- ✅ `src/hooks/useApi.js` - API data fetching hook with loading/error states (includes useMutation)
- ✅ `src/hooks/useToast.js` - Toast notification management hook
- ✅ `src/hooks/usePagination.js` - Pagination logic hook
- ✅ `src/hooks/useResponsive.js` - Responsive breakpoint detection (includes useMediaQuery, useOrientation)
- ✅ `src/hooks/useLocalStorage.js` - localStorage persistence hook with JSON serialization (includes useSessionStorage)
- ✅ `src/hooks/useDebounce.js` - Debounce utility hook (includes useDebouncedCallback, useThrottle, useThrottledCallback)
- ✅ `src/hooks/index.js` - Centralized exports for all hooks

**Acceptance Criteria:**
- ✅ All hooks follow React best practices
- ✅ All hooks handle cleanup properly with useEffect cleanup functions
- ✅ All hooks are reusable across components
- ✅ All hooks have proper JSDoc comments for type safety
- ✅ Edge cases handled (null values, network errors, SSR compatibility, unmounted components)

---

#### **✅ TASK A5: Utility Functions** [COMPLETED]
**Agent:** Utils Specialist
**Estimated Time:** 4 hours
**Files Created:**
- ✅ `src/utils/format.js` - Formatting utilities (formatBytes, formatDuration, formatTimestamp, formatRelativeTime, formatNumber, formatPercentage)
- ✅ `src/utils/validation.js` - Form validation (email, URL, cron, port, IPv4, JSON, domain, phone, password)
- ✅ `src/utils/responsive.js` - Responsive detection (isMobile, isTablet, isDesktop, getCurrentBreakpoint, isTouchDevice, etc.)
- ✅ `src/utils/clipboard.js` - Clipboard operations (copyToClipboard, readFromClipboard, isClipboardSupported)
- ✅ `src/utils/toast.js` - Toast helpers (showToast, showSuccessToast, showErrorToast, showWarningToast, showInfoToast, clearAllToasts)
- ✅ `src/utils/debounce.js` - Debounce and throttle utilities
- ✅ `src/utils/theme.js` - Theme management (setTheme, getTheme, toggleTheme, initializeTheme, watchSystemTheme)
- ✅ `src/utils/index.js` - Centralized exports for all utilities

**Acceptance Criteria:**
- ✅ All utilities from `utils.js` migrated (formatBytes, formatDuration, showToast, responsive detection, clipboard, debounce, theme)
- ✅ All utilities are tree-shakeable with named exports
- ✅ All functions have JSDoc comments for documentation
- ✅ No external dependencies used

---

#### **✅ TASK A6: State Management Setup** [COMPLETED]
**Agent:** State Management Specialist
**Estimated Time:** 4 hours
**Files Created:**
- ✅ `src/store/dashboardStore.js` - Dashboard state (servers, status, metrics, selected server)
- ✅ `src/store/chatStore.js` - Chat state (sessions, messages, active session, streaming state)
- ✅ `src/store/uiStore.js` - UI state (theme, mobile menu, sidebar, modals, toasts)

**Acceptance Criteria:**
- ✅ Zustand stores are properly typed with TypeScript-style JSDoc comments
- ✅ State updates are optimized with optimized selectors (no unnecessary re-renders)
- ✅ Store actions are well-documented with JSDoc comments
- ✅ DevTools integration works (zustand/middleware devtools)
- ✅ Theme preference persisted to localStorage (persist middleware)
- ✅ All stores have proper action tracking for debugging

---

### Task Group B: Core Dashboard (Depends on: A1-A6)

#### **✅ TASK B1: Main Dashboard Layout & Server Overview** [COMPLETED]
**Agent:** Dashboard Component Specialist
**Estimated Time:** 16 hours
**Source File:** `dashboard.js` (2,718 lines)
**Files Created:**
- ✅ `src/components/Dashboard/Dashboard.jsx` - Main dashboard with tab navigation and real-time updates
- ✅ `src/components/Dashboard/ServerCard.jsx` - Individual server display with expand/collapse functionality
- ✅ `src/components/Dashboard/ServerMetrics.jsx` - Stats cards with total/running/healthy/uptime/connections
- ✅ `src/components/Dashboard/ServerFilters.jsx` - Search input and filter dropdowns
- ✅ `src/components/Dashboard/ServerActions.jsx` - Start/stop/restart/logs action buttons
- ✅ `src/components/Dashboard/ProxyControls.jsx` - Proxy restart with confirmation modal
- ✅ `src/components/Dashboard/MobileMenu.jsx` - Hamburger menu for mobile navigation
- ✅ `src/components/Dashboard/index.js` - Centralized exports

**Features Migrated:**
- ✅ Server overview with animated status cards and gradient backgrounds
- ✅ Server metrics (total, running, healthy, connections, uptime) with icons and pulse animations
- ✅ Server actions (start, stop, restart) with loading states and toast notifications
- ✅ Real-time status updates via WebSocket integration
- ✅ Search & filter (by name, status) with SearchInput component
- ✅ Sort options (by name, status, health, tools) with Select dropdown
- ✅ Auto-refresh with configurable intervals (5s, 10s, 30s, 1m, 5m) and dropdown settings
- ✅ Proxy restart functionality with confirmation modal
- ✅ Mobile hamburger menu with action buttons
- ✅ Server expand/collapse with capabilities and tools display
- ✅ Connection status badges and health indicators with pulsing animations

**Acceptance Criteria:**
- ✅ All server data displays correctly from API
- ✅ Server actions trigger correct API calls (startServer, stopServer, restartServer)
- ✅ Real-time updates work via WebSocket with useWebSocket hook
- ✅ Search and filtering work correctly using dashboardStore selectors
- ✅ Mobile hamburger menu functions properly with close on action
- ✅ All touch targets meet 44×44px minimum (buttons, icons, interactive elements)
- ✅ Component is fully responsive (mobile-first with sm/md/lg/xl breakpoints)
- ✅ Dark mode works correctly throughout with slate/gray color scheme
- ✅ Uses dashboardStore from Zustand for state management
- ✅ Uses dashboard API for server actions
- ✅ Uses useWebSocket hook for real-time updates
- ✅ All shared components used (Button, Modal, Badge, Spinner, EmptyState, SearchInput, Select)

---

#### **✅ TASK B2: Chat Interface** [COMPLETED]
**Agent:** Chat Component Specialist
**Estimated Time:** 14 hours
**Source File:** `chat.js` (1,021 lines)
**Files Created:**
- ✅ `src/components/Chat/Chat.jsx` - Main chat interface with WebSocket integration
- ✅ `src/components/Chat/SessionList.jsx` - Chat session sidebar with create/delete/rename
- ✅ `src/components/Chat/MessageList.jsx` - Message history display with empty state
- ✅ `src/components/Chat/ChatInput.jsx` - Message input with auto-resize and keyboard shortcuts
- ✅ `src/components/Chat/Message.jsx` - Individual message with markdown and syntax highlighting
- ✅ `src/components/Chat/ToolCall.jsx` - Tool execution visualization with collapsible results
- ✅ `src/components/Chat/ModelSelector.jsx` - AI provider and model selection dropdowns
- ✅ `src/components/Chat/MCPServerSelector.jsx` - MCP server selection dropdown with bulk actions
- ✅ `src/components/Chat/ConnectionStatus.jsx` - WebSocket connection status indicator
- ✅ `src/components/Chat/index.js` - Centralized exports

**Features Migrated:**
- ✅ Multi-session chat management (create, load, delete, switch)
- ✅ AI provider selection (OpenAI, Anthropic, Ollama, OpenRouter)
- ✅ Model selection per provider with dropdown
- ✅ MCP server selection for tool access with checkboxes
- ✅ Streaming responses via WebSocket with useWebSocket hook
- ✅ Message history display with persistence
- ✅ Tool execution visualization with parameters and results
- ✅ Session CRUD (create, rename, delete)
- ✅ System prompt viewer with collapsible panel
- ✅ Markdown rendering with react-markdown
- ✅ Code block syntax highlighting with react-syntax-highlighter (vscDarkPlus theme)
- ✅ Mobile sidebar with session list (collapsible, hamburger menu)
- ✅ WebSocket connection status indicator with visual feedback
- ✅ Empty state with suggestion chips
- ✅ Error handling with dismissible error messages
- ✅ Auto-resize textarea with Shift+Enter for new lines

**Acceptance Criteria:**
- ✅ Chat sessions load correctly from API
- ✅ Messages stream in real-time via WebSocket
- ✅ Tool calls display with parameters and results in collapsible accordion
- ✅ Session management works (create, delete, rename, switch)
- ✅ MCP server selection updates chat context via API
- ✅ WebSocket connection status shows correctly (connected/disconnected)
- ✅ Mobile sidebar collapses/expands properly with hamburger menu
- ✅ Markdown renders correctly with react-markdown
- ✅ Code blocks use syntax highlighting (vscDarkPlus theme)
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout
- ✅ Uses chatStore from Zustand for state management
- ✅ Uses chatApi for API calls
- ✅ Uses useWebSocket hook for real-time streaming
- ✅ All shared components used (Button, Modal, EmptyState, Badge, Checkbox, Select)

---

#### **✅ TASK B3: MCP Inspector** [COMPLETED]
**Agent:** Inspector Component Specialist
**Estimated Time:** 8 hours
**Source File:** `inspector.js` (455 lines)
**Files Created:**
- ✅ `src/components/Inspector/Inspector.jsx` - Main inspector interface
- ✅ `src/components/Inspector/ServerSelector.jsx` - Server dropdown selector
- ✅ `src/components/Inspector/RequestEditor.jsx` - JSON-RPC 2.0 request editor with validation
- ✅ `src/components/Inspector/ResponseViewer.jsx` - JSON formatted response display with copy
- ✅ `src/components/Inspector/TemplateSelector.jsx` - Pre-built request templates dropdown
- ✅ `src/components/Inspector/ToolList.jsx` - Discovered tools display with test action
- ✅ `src/components/Inspector/index.js` - Centralized exports

**Features Migrated:**
- ✅ Server connection selector with dropdown
- ✅ MCP request editor with JSON-RPC 2.0 validation
- ✅ Request templates (initialize, tools/list, resources/list, prompts/list, tools/call, resources/read, prompts/get)
- ✅ Response viewer with JSON syntax highlighting and copy button
- ✅ Tool discovery with automatic execution on connect
- ✅ Availability checking with graceful fallback for unavailable endpoints
- ✅ Auto-connect to first available server on mount
- ✅ Quick action buttons for discovered methods
- ✅ Ctrl+Enter / Cmd+Enter keyboard shortcut for sending requests
- ✅ Tool testing with pre-populated templates

**Acceptance Criteria:**
- ✅ Server selection works via dropdown
- ✅ Request editor sends valid MCP requests with validation
- ✅ Response viewer formats JSON correctly with color coding (green for success, red for errors)
- ✅ Request templates populate editor correctly
- ✅ Tool discovery runs automatically on connection
- ✅ Component handles unavailable endpoints gracefully with empty state
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout
- ✅ Proper error handling with toast notifications
- ✅ Uses useApi hook (useMutation) for data fetching
- ✅ All shared components used (Button, Select, Badge, Spinner, EmptyState, Card)

---

### Task Group C: Advanced Features (Depends on: A1-A6, B1)

#### **✅ TASK C1: Task Scheduler** [COMPLETED]
**Agent:** Task Scheduler Specialist
**Estimated Time:** 18 hours
**Source File:** `task-scheduler.js` (3,136 lines)
**Files Created:**
- ✅ `src/store/taskStore.js` - Zustand state management for tasks
- ✅ `src/components/TaskScheduler/TaskScheduler.jsx` - Main component with tabs, stats, and auto-refresh
- ✅ `src/components/TaskScheduler/TaskList.jsx` - Task list with grouped display
- ✅ `src/components/TaskScheduler/TaskCard.jsx` - Individual task card with expandable details
- ✅ `src/components/TaskScheduler/TaskForm.jsx` - Create/edit task form with type-specific fields
- ✅ `src/components/TaskScheduler/TaskGroup.jsx` - Collapsible task group by type
- ✅ `src/components/TaskScheduler/TaskStats.jsx` - Statistics cards (total, enabled, running, completed/failed 24h)
- ✅ `src/components/TaskScheduler/CronEditor.jsx` - Cron expression editor with presets
- ✅ `src/components/TaskScheduler/TaskOutput.jsx` - Task execution output modal
- ✅ `src/components/TaskScheduler/constants.js` - Task types, cron presets, model hints
- ✅ `src/components/TaskScheduler/index.js` - Centralized exports

**Features Migrated:**
- ✅ Task types: shell, AI, manual, dependency, watcher with type-specific forms
- ✅ Task CRUD operations (create, edit, delete, enable/disable)
- ✅ Cron scheduling with presets (every minute, hourly, daily, weekly, monthly, custom)
- ✅ Task execution (manual trigger with confirmation)
- ✅ Task output viewing (latest output + specific run output in modal)
- ✅ Task history with status badges and duration (recent 5 runs displayed)
- ✅ Task statistics (total, enabled, running, completed/failed 24h) with gradient stat cards
- ✅ Task groups organized by type with accordion UI
- ✅ Search & filter (by name, description, command, prompt) with SearchInput
- ✅ Sort options (by name, type, status, schedule, last run) with Select dropdown
- ✅ Auto-refresh with 30s interval toggle
- ✅ Cron descriptions (human-readable schedule descriptions)
- ✅ Task configuration display (command/prompt, schedule, AI settings, dependencies)
- ✅ Recent runs display with status badges and view output buttons
- ✅ Copy to clipboard for commands/prompts
- ✅ Empty state with create task button
- ✅ Error handling with dismissible error messages

**Acceptance Criteria:**
- ✅ All 5 task types can be created (shell, AI, manual, dependency, watcher)
- ✅ Cron editor works with presets and displays human-readable descriptions
- ✅ Task execution triggers correctly with confirmation dialog
- ✅ Task output displays correctly in modal with copy button
- ✅ Task history shows recent runs with status badges
- ✅ Statistics calculate correctly (24h window for completed/failed)
- ✅ Search and filter work with real-time updates
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout
- ✅ Uses taskStore from Zustand for state management
- ✅ Uses task API for data fetching and actions
- ✅ All shared components used (Button, Modal, SearchInput, Select, Card, Badge, Spinner, EmptyState, Input, Checkbox)

---

#### **✅ TASK C2: Memory Management** [COMPLETED]
**Agent:** Memory Component Specialist
**Estimated Time:** 16 hours
**Source File:** `memory.js` (2,638 lines)
**Files Created:**
- ✅ `src/store/memoryStore.js` - Zustand state management for memory
- ✅ `src/components/Memory/Memory.jsx` - Main component with tab navigation and real-time updates
- ✅ `src/components/Memory/EntityList.jsx` - Paginated entity list with filters and bulk actions
- ✅ `src/components/Memory/EntityCard.jsx` - Individual entity card with expand/collapse functionality
- ✅ `src/components/Memory/EntityForm.jsx` - Create entity form modal with observations
- ✅ `src/components/Memory/RelationForm.jsx` - Create relationship form modal
- ✅ `src/components/Memory/ObservationList.jsx` - Observation list with add/delete functionality
- ✅ `src/components/Memory/SearchView.jsx` - Advanced search interface with filters
- ✅ `src/components/Memory/GraphView.jsx` - Graph visualization placeholder with network overview
- ✅ `src/components/Memory/AnalyticsView.jsx` - Statistics and analytics dashboard
- ✅ `src/components/Memory/index.js` - Centralized exports

**Features Migrated:**
- ✅ Entity CRUD operations (create, read, delete) with API integration
- ✅ Entity type classification (person, organization, event, concept, etc.)
- ✅ Observation management (add/delete observations per entity)
- ✅ Relationship creation/deletion between entities
- ✅ Graph visualization placeholder (network overview with stats, planned D3.js/vis.js implementation)
- ✅ Full-text search across entities and observations with advanced filters
- ✅ Filtering by entity type, date range with Select and date inputs
- ✅ Pagination (50 entities per page) with Pagination component
- ✅ Bulk operations (multi-select delete) with selection state management
- ✅ Statistics (entity count, relation count, type distribution) with gradient cards
- ✅ Tab navigation (Browse, Search, Visualization, Analytics) with active state
- ✅ Auto-refresh capability with loading states
- ✅ Empty states with helpful messages and create actions
- ✅ Error handling with dismissible error messages and toast notifications

**Acceptance Criteria:**
- ✅ Entity CRUD works correctly with API integration (create, delete via memoryApi)
- ✅ Observations can be added/deleted per entity with inline editing
- ✅ Relationships can be created between entities with dropdown selectors
- ✅ Search works across all fields (name, type, observations) with debounced input
- ✅ Filtering works correctly (entity type, search query, date range)
- ✅ Pagination works with proper page navigation and totals
- ✅ Bulk delete works with multi-select checkboxes and confirmation
- ✅ Statistics display correctly (total entities, relations, type distributions, connectivity)
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout with gradient backgrounds
- ✅ Uses memoryStore from Zustand for state management
- ✅ Uses memory API for data fetching (getMemoryStats, createEntity, deleteEntity, etc.)
- ✅ All shared components used (Button, Modal, SearchInput, Select, Card, Badge, EmptyState, Spinner, Pagination, Input)

---

#### **✅ TASK C3: Activity Monitor** [COMPLETED]
**Agent:** Activity Component Specialist
**Estimated Time:** 12 hours
**Source File:** `activity.js` (856 lines)
**Files Created:**
- ✅ `src/components/Activity/Activity.jsx` - Main component with WebSocket integration
- ✅ `src/components/Activity/ActivityList.jsx` - Scrollable activity feed with auto-scroll
- ✅ `src/components/Activity/ActivityCard.jsx` - Individual activity event with badges
- ✅ `src/components/Activity/ActivityFilters.jsx` - Filter controls (level, type, search)
- ✅ `src/components/Activity/ActivityStats.jsx` - Statistics cards (total, requests, tools, errors)
- ✅ `src/components/Activity/ToolCallDetails.jsx` - Expandable tool call details
- ✅ `src/components/Activity/index.js` - Centralized exports
- ✅ `src/store/activityStore.js` - Zustand store for activity state

**Features Migrated:**
- ✅ Real-time activity stream via WebSocket with useWebSocket hook
- ✅ Historical data loading (6 hours from PostgreSQL) with API client
- ✅ Event types: requests, connections, tool calls, errors
- ✅ Event levels: ERROR, WARN, INFO, DEBUG with color-coded badges
- ✅ Statistics: total events, requests, tool calls, errors with gradient cards
- ✅ Filtering by level, type, search term with Select and SearchInput components
- ✅ Tool call expansion to show parameters/results with collapsible accordion
- ✅ Auto-scroll to new events with useRef and useEffect
- ✅ Clear activity feed button with confirmation toast
- ✅ Merge historical and real-time events with duplicate detection
- ✅ Empty state with friendly message
- ✅ Loading state with spinner
- ✅ Connection status indicator (connected/disconnected)
- ✅ Refresh button with loading state

**Acceptance Criteria:**
- ✅ Real-time events stream correctly via WebSocket
- ✅ Historical events load correctly from API
- ✅ Merge of historical and real-time events works with deduplication
- ✅ Filtering works correctly (level, type, search)
- ✅ Tool call details expand correctly with toggle state
- ✅ Statistics calculate correctly (real-time + historical)
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout
- ✅ Uses activityStore from Zustand for state management
- ✅ Uses activityApi for API calls
- ✅ Uses useWebSocket hook for real-time updates
- ✅ All shared components used (Button, Badge, Select, SearchInput, Spinner)
---

#### **✅ TASK C4: Log Viewer** [COMPLETED]
**Agent:** Log Viewer Specialist
**Estimated Time:** 10 hours
**Source File:** `logs.js` (783 lines)
**Files Created:**
- ✅ `src/store/logsStore.js` - Zustand state management for logs
- ✅ `src/components/Logs/Logs.jsx` - Main log viewer with WebSocket integration
- ✅ `src/components/Logs/LogList.jsx` - Virtual scrolling log list with react-window
- ✅ `src/components/Logs/LogLine.jsx` - Individual log line with syntax highlighting and search highlight
- ✅ `src/components/Logs/LogControls.jsx` - Control buttons, server selector, filters, and display options
- ✅ `src/components/Logs/LogStats.jsx` - Log statistics with gradient cards
- ✅ `src/components/Logs/TerminalWindow.jsx` - macOS-style terminal window chrome with traffic lights
- ✅ `src/components/Logs/index.js` - Centralized exports

**Features Migrated:**
- ✅ Container log viewing (Docker/Podman logs) with API integration
- ✅ Server selection dropdown with server list
- ✅ Real-time log streaming via WebSocket with useWebSocket hook
- ✅ Log level auto-detection (ERROR, WARN, INFO, DEBUG) with detectLogLevel function
- ✅ Log actions: load last 100 lines, start/stop streaming, clear, download as .txt
- ✅ Search logs by term with highlighting in LogLine
- ✅ Filter by log level (all, ERROR, WARN, INFO, DEBUG)
- ✅ Display options: show timestamps, auto-scroll, line wrap with checkboxes
- ✅ Terminal UI with macOS-style window chrome (red/yellow/green traffic lights)
- ✅ Syntax highlighting for log levels (red for ERROR, yellow for WARN, cyan for INFO, purple for DEBUG)
- ✅ Virtual scrolling for performance (react-window) when not wrapping
- ✅ Log statistics display with counts per level
- ✅ Scroll to top/bottom buttons in terminal header
- ✅ Max 1000 logs in memory with automatic trimming

**Acceptance Criteria:**
- ✅ Log streaming works correctly via WebSocket
- ✅ Server selection updates logs and reloads
- ✅ Log level detection works with keyword matching
- ✅ Search and filter work with real-time filtering
- ✅ Download logs works with .txt format and timestamp filename
- ✅ Terminal UI renders correctly with macOS-style chrome
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout with terminal color scheme
- ✅ Virtual scrolling for performance with react-window
- ✅ Uses logsStore from Zustand for state management
- ✅ Uses useWebSocket hook for real-time streaming
- ✅ All shared components used (Button, Select, SearchInput, EmptyState)

---

### Task Group D: Security & Administration (Depends on: A1-A6)

#### **✅ TASK D1: OAuth Configuration** [COMPLETED]
**Agent:** OAuth Component Specialist
**Estimated Time:** 14 hours
**Source File:** `oauth.js` (1,499 lines)
**Files Created:**
- ✅ `src/store/oauthStore.js` - Zustand state management for OAuth
- ✅ `src/components/OAuth/OAuth.jsx` - Main OAuth component with tabs and auto-refresh
- ✅ `src/components/OAuth/OAuthStatus.jsx` - OAuth server status display with expandable token stats
- ✅ `src/components/OAuth/ClientList.jsx` - Client list with search and filter
- ✅ `src/components/OAuth/ClientCard.jsx` - Individual client card with actions
- ✅ `src/components/OAuth/ClientForm.jsx` - Create/edit client form modal
- ✅ `src/components/OAuth/ClientDetails.jsx` - Client details modal with secret visibility toggle
- ✅ `src/components/OAuth/EndpointList.jsx` - OAuth endpoint display with copy buttons
- ✅ `src/components/OAuth/TestFlows.jsx` - OAuth flow testing interface
- ✅ `src/components/OAuth/index.js` - Centralized exports

**Features Migrated:**
- ✅ OAuth server status display (enabled/disabled) with expandable section
- ✅ Token statistics (active access tokens, refresh tokens, auth codes) with gradient stat cards
- ✅ Client management (register, view, delete) with confirmation dialogs
- ✅ Client types (public vs. confidential) with type badges
- ✅ Client details modal showing client ID, secret (with show/hide toggle), redirect URIs
- ✅ OAuth endpoint display (authorization, token, discovery endpoints) with copy to clipboard
- ✅ Test flows (authorization code flow, client credentials flow) with error handling
- ✅ Search & filter clients (by name, type) with SearchInput and Select components
- ✅ Client statistics (total, public count, confidential count, active tokens) with stat cards
- ✅ Copy to clipboard for client ID/secret/endpoints with toast notifications
- ✅ Redirect URI list management (display in client details, add via form)
- ✅ Empty state for no clients with create button
- ✅ Error handling with dismissible error messages
- ✅ Auto-refresh capability (30s interval) with state management

**Acceptance Criteria:**
- ✅ OAuth status displays correctly with token counts
- ✅ Client CRUD works correctly (create via register endpoint, delete via API)
- ✅ Client details modal shows all info (ID, secret with toggle, URIs, scopes)
- ✅ OAuth endpoints are copyable with toast confirmation
- ✅ Test flows execute correctly (authorization code redirects, client credentials API call)
- ✅ Statistics display correctly (total, public, confidential, active tokens)
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout with gradient backgrounds
- ✅ Uses oauthStore from Zustand for state management
- ✅ Uses oauth API for data fetching (getOAuthStatus, getOAuthClients, registerOAuthClient, deleteOAuthClient)
- ✅ All shared components used (Button, Modal, Badge, EmptyState, SearchInput, Select, Input, Checkbox, Spinner)

---

#### **✅ TASK D2: Audit Log Viewer** [COMPLETED]
**Agent:** Audit Component Specialist
**Estimated Time:** 12 hours
**Source File:** `audit.js` (1,165 lines)
**Files Created:**
- ✅ `src/store/auditStore.js` - Zustand state management for audit logs
- ✅ `src/components/Audit/Audit.jsx` - Main audit log viewer with filters and auto-refresh
- ✅ `src/components/Audit/AuditList.jsx` - Paginated audit log list with sorting
- ✅ `src/components/Audit/AuditEntry.jsx` - Individual audit entry with expandable details
- ✅ `src/components/Audit/AuditFilters.jsx` - Filter controls (event type, success/failure, time range, search)
- ✅ `src/components/Audit/AuditStats.jsx` - Statistics cards (total, success rate, success/failure counts)
- ✅ `src/components/Audit/EventChart.jsx` - Simple bar chart for event distribution
- ✅ `src/components/Audit/ExportButton.jsx` - CSV export with filtering
- ✅ `src/components/Audit/constants.js` - Event types and time range options
- ✅ `src/components/Audit/utils.js` - Utility functions (formatTimestamp, getEventIcon, etc.)
- ✅ `src/components/Audit/index.js` - Centralized exports

**Features Migrated:**
- ✅ Audit event display with expandable details in modal
- ✅ Event types: token issued/revoked, login/logout, access granted/denied, client created/deleted, config changes
- ✅ Statistics: total events, success rate, success/failure counts with gradient stat cards
- ✅ Event distribution chart (simple Tailwind CSS bar chart by event type)
- ✅ Filtering: by event type, success/failure, time range (1h, 24h, 7d, 30d, all)
- ✅ Full-text search across audit entries with debounced SearchInput
- ✅ Pagination with page size options (20 per page default)
- ✅ CSV export with proper formatting and download
- ✅ Auto-refresh with configurable 30s interval
- ✅ Table sorting by timestamp, event, success status
- ✅ Empty state with clear filters action
- ✅ Error handling with dismissible error messages

**Acceptance Criteria:**
- ✅ Audit entries load correctly from API
- ✅ Filtering works correctly (event type, success/failure, time range, search)
- ✅ Search works across all fields with debounce
- ✅ Pagination works with proper page navigation
- ✅ CSV export works with filtered data and timestamp filename
- ✅ Statistics calculate correctly (total, success rate, counts)
- ✅ Event distribution chart displays top 10 events with percentage bars
- ✅ Component is fully responsive (mobile-first with Tailwind breakpoints)
- ✅ All touch targets meet 44×44px minimum
- ✅ Dark mode support throughout with slate color scheme
- ✅ Uses auditStore from Zustand for state management
- ✅ Uses audit API for data fetching
- ✅ All shared components used (Button, Modal, SearchInput, Select, Pagination, Card, EmptyState, Spinner)

---

### Task Group E: Polish & Testing (Depends on: All previous tasks)

#### **TASK E1: Mobile Responsiveness Testing**
**Agent:** QA Mobile Specialist
**Estimated Time:** 12 hours
**Devices to Test:**
- iPhone SE (375×667)
- iPhone 12/13/14 (390×844)
- iPhone 14 Pro Max (430×932)
- Samsung Galaxy S21 (360×800)
- iPad (768×1024)
- iPad Pro (1024×1366)

**Acceptance Criteria:**
- All components render correctly on all devices
- All touch targets meet 44×44px minimum
- No horizontal scrolling on any device
- All modals/dialogs fit on screen
- All forms are usable on mobile
- Navigation works on all devices

---

#### **TASK E2: Cross-Browser Testing**
**Agent:** QA Browser Specialist
**Estimated Time:** 8 hours
**Browsers to Test:**
- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)
- Safari iOS (latest)
- Chrome Android (latest)

**Acceptance Criteria:**
- All features work on all browsers
- No JavaScript errors in console
- CSS renders correctly on all browsers
- WebSocket connections work on all browsers

---

#### **TASK E3: Performance Optimization**
**Agent:** Performance Specialist
**Estimated Time:** 10 hours
**Tasks:**
- Analyze bundle size and split code
- Implement lazy loading for heavy components
- Optimize images and assets
- Implement React.memo where appropriate
- Add performance monitoring
- Run Lighthouse audits

**Acceptance Criteria:**
- FCP < 1.8s
- LCP < 2.5s
- FID < 100ms
- CLS < 0.1
- Initial JS bundle < 200KB gzipped
- Lighthouse score > 90

---

#### **TASK E4: Accessibility Audit**
**Agent:** Accessibility Specialist
**Estimated Time:** 10 hours
**Tasks:**
- Run axe DevTools audit
- Test with screen readers (NVDA, JAWS, VoiceOver)
- Test keyboard navigation
- Check color contrast
- Add ARIA labels where needed
- Fix semantic HTML issues

**Acceptance Criteria:**
- WCAG 2.1 Level AA compliant
- No axe DevTools violations
- All interactive elements keyboard accessible
- All images have alt text
- All forms have proper labels
- Color contrast meets 4.5:1 minimum

---

#### **TASK E5: User Acceptance Testing**
**Agent:** QA Lead
**Estimated Time:** 8 hours
**Tasks:**
- Create test scenarios for all features
- Execute test scenarios on staging environment
- Document bugs and issues
- Verify bug fixes
- Sign off on production readiness

**Acceptance Criteria:**
- All test scenarios pass
- No critical bugs
- All user stories are satisfied
- Stakeholder approval obtained

---

## 7. Component Dependencies

### Dependency Graph

```
Foundation (A1-A6)
    ├── Build System (A1)
    ├── Shared Components (A2)
    ├── API Client (A3)
    ├── Custom Hooks (A4)
    ├── Utilities (A5)
    └── State Management (A6)
        │
        ├── Core Dashboard (Task Group B)
        │   ├── Dashboard Layout (B1)
        │   ├── Chat Interface (B2)
        │   └── MCP Inspector (B3)
        │
        ├── Advanced Features (Task Group C)
        │   ├── Task Scheduler (C1)
        │   ├── Memory Management (C2)
        │   ├── Activity Monitor (C3)
        │   └── Log Viewer (C4)
        │
        ├── Security & Admin (Task Group D)
        │   ├── OAuth Configuration (D1)
        │   └── Audit Logs (D2)
        │
        └── Polish & Testing (Task Group E)
            ├── Mobile Testing (E1)
            ├── Browser Testing (E2)
            ├── Performance (E3)
            ├── Accessibility (E4)
            └── UAT (E5)
```

### Parallel Execution Strategy

**Week 1:**
- **5 agents in parallel:** Execute all Task Group A tasks (A1-A6)
- Expected completion: Foundation ready

**Week 2:**
- **3 agents in parallel:** Execute Task Group B (B1, B2, B3)
- Expected completion: Core dashboard functional

**Week 3:**
- **4 agents in parallel:** Execute Task Group C (C1, C2, C3, C4)
- Expected completion: All advanced features migrated

**Week 4:**
- **2 agents in parallel:** Execute Task Group D (D1, D2)
- Expected completion: Security features migrated

**Week 5:**
- **5 agents in parallel:** Execute Task Group E (E1, E2, E3, E4, E5)
- Expected completion: Production-ready

---

## 8. Acceptance Criteria

### 8.1 Functional Criteria

✅ **All features from Vue.js version work identically in React**
✅ **All API endpoints integrate correctly**
✅ **WebSocket connections work for real-time features**
✅ **Data persistence works (localStorage, PostgreSQL)**
✅ **All CRUD operations function correctly**
✅ **Search and filtering work across all components**
✅ **Pagination works correctly**
✅ **Forms validate correctly**
✅ **Toast notifications display correctly**
✅ **Modals/dialogs open and close correctly**

### 8.2 Mobile-First Criteria

✅ **Mobile-first CSS approach used throughout**
✅ **All components responsive from 320px to 2560px**
✅ **All touch targets ≥ 44×44px**
✅ **No horizontal scrolling on any device**
✅ **Mobile navigation works (hamburger menu, bottom tabs)**
✅ **Forms are mobile-friendly (large inputs, proper input types)**
✅ **Performance is good on mobile devices (FCP < 1.8s)**

### 8.3 Visual Criteria

✅ **Dark mode works throughout the application**
✅ **Theme persists across sessions**
✅ **Animations are smooth (60fps)**
✅ **Loading states are clear and consistent**
✅ **Empty states are user-friendly**
✅ **Error states provide actionable feedback**
✅ **Typography is readable on all devices**
✅ **Color contrast meets WCAG 2.1 AA (4.5:1)**

### 8.4 Technical Criteria

✅ **Build completes without errors**
✅ **No console errors in browser**
✅ **No React warnings in development**
✅ **Bundle size < 200KB gzipped (initial)**
✅ **Code splitting implemented for heavy components**
✅ **Lighthouse score > 90**
✅ **TypeScript errors resolved (if using TS)**

### 8.5 Accessibility Criteria

✅ **WCAG 2.1 Level AA compliant**
✅ **Keyboard navigation works throughout**
✅ **Screen reader support (tested with NVDA, JAWS, VoiceOver)**
✅ **Focus indicators visible on all interactive elements**
✅ **ARIA attributes correct**
✅ **Semantic HTML used throughout**
✅ **Forms have proper labels**
✅ **Images have alt text**

### 8.6 Browser Compatibility Criteria

✅ **Works in Chrome (latest)**
✅ **Works in Firefox (latest)**
✅ **Works in Safari (latest)**
✅ **Works in Edge (latest)**
✅ **Works in Safari iOS (latest)**
✅ **Works in Chrome Android (latest)**

---

## Summary

This migration plan provides a comprehensive roadmap for transitioning the MCP Compose Dashboard from Vue 3 to React.js with a mobile-first approach. The plan is organized into 25 distinct tasks across 5 task groups, enabling parallel execution by multiple agents.

**Total Estimated Time:** ~200 hours across 5 weeks with 5 parallel agents

**Key Success Metrics:**
- ✅ All 11 Vue components migrated to React
- ✅ Mobile-first responsive design implemented
- ✅ WCAG 2.1 Level AA accessibility achieved
- ✅ Performance benchmarks met (Core Web Vitals)
- ✅ Cross-browser compatibility verified
- ✅ Production-ready deployment

**Next Steps:**
1. Review and approve this migration plan
2. Assign agents to task groups
3. Set up project tracking (GitHub Projects, Jira, etc.)
4. Begin Week 1 (Task Group A: Foundation)
5. Daily standups to track progress and resolve blockers

---

**Document Version:** 1.0
**Last Updated:** 2025-01-XX
**Author:** Claude Code
**Status:** Draft - Awaiting Review
