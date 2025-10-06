# MCP Compose Dashboard - React Frontend

Modern React 18 application for managing and monitoring Model Context Protocol servers.

## Quick Start

### Development

```bash
npm install
npm run dev
```

The application will be available at `http://localhost:3000`

### Production Build

```bash
npm run build
npm run preview
```

## Architecture

### Tech Stack

- React 18 (with createRoot)
- Vite (build tool)
- Tailwind CSS (styling)
- Zustand (state management)
- Headless UI (accessible components)
- Heroicons (icons)

### Features

- Tab-based navigation with 9 main sections
- Dark mode support with persistent theme selection
- Mobile-responsive with hamburger menu
- Code splitting with React.lazy() and Suspense
- Toast notifications system
- Real-time server monitoring
- MCP protocol inspector
- Task scheduling interface
- Memory management
- Activity logging
- OAuth integration
- Audit trail

### Component Structure

```
src/
├── App.jsx                    # Main application with tab navigation
├── main.jsx                   # Entry point
├── components/
│   ├── Dashboard/            # Server overview and metrics
│   ├── Chat/                 # Chat interface
│   ├── Inspector/            # MCP protocol inspector
│   ├── TaskScheduler/        # Cron task management
│   ├── Memory/               # Memory management
│   ├── Activity/             # Activity logs
│   ├── Logs/                 # System logs
│   ├── OAuth/                # OAuth configuration
│   ├── Audit/                # Audit trail
│   └── shared/               # Reusable components
├── hooks/                    # Custom React hooks
├── utils/                    # Utility functions
├── api/                      # API client
├── store/                    # Zustand stores
└── styles/                   # Global styles
```

## Key Components

### Dashboard
Server status, metrics, and health monitoring

### Chat
Interactive chat interface with MCP servers

### Inspector
Debug MCP protocol messages and server capabilities

### Task Scheduler
Cron-style task scheduling with execution history

### Memory
Manage persistent memory and context storage

### Activity
Real-time activity feed and tool call monitoring

### Logs
System logs with filtering and search

### OAuth
OAuth provider configuration and token management

### Audit
Security audit trail and access logs

## Development

### Code Splitting
All tab components are lazy-loaded for optimal performance:

```jsx
const Dashboard = lazy(() => import('./components/Dashboard'));
```

### Theme System
Theme is managed via localStorage and CSS classes:

```jsx
import { initializeTheme, toggleTheme } from './utils/theme';
```

### Toast Notifications
Use the toast hook for user feedback:

```jsx
import { useToast } from './components/shared/Toast';

const { toast } = useToast();
toast.success('Operation completed');
toast.error('Operation failed');
```

## Mobile Support

- Responsive design (mobile-first approach)
- Touch targets ≥ 44×44px for accessibility
- Hamburger menu for navigation on small screens
- Safe area insets for notched devices
- iOS momentum scrolling

## Browser Support

- Modern browsers (Chrome, Firefox, Safari, Edge)
- ES2015+ support required
- CSS Grid and Flexbox support required
- WebSocket support required

## Environment Variables

Create a `.env.local` file for local configuration:

```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws
```

## Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

## Performance

- Code splitting per tab
- Lazy loading of components
- React.memo() for expensive components
- Virtual scrolling for large lists (react-window)
- Debounced search inputs
- Optimized re-renders with Zustand

## Accessibility

- WCAG 2.1 Level AA compliance
- Keyboard navigation support
- Screen reader friendly
- Focus management
- ARIA labels and roles
- Color contrast ratios
- Reduced motion support

## Testing

```bash
npm run test
```

## License

See main project LICENSE file.
