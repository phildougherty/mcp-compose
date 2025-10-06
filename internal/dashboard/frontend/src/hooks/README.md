# Custom React Hooks

This directory contains reusable React hooks for the MCP Compose Dashboard.

## Available Hooks

### useWebSocket
WebSocket connection management with automatic reconnection.

```javascript
import { useWebSocket } from './hooks';

function Component() {
  const { isConnected, send, lastMessage } = useWebSocket('ws://localhost:8080/ws', {
    onMessage: (data) => console.log('Received:', data),
    reconnectDelay: 3000,
    maxReconnectAttempts: 5
  });

  return <div>Connected: {isConnected ? 'Yes' : 'No'}</div>;
}
```

### useApi
Data fetching with loading and error states.

```javascript
import { useApi, useMutation } from './hooks';

function Component() {
  const { data, loading, error, refetch } = useApi(fetchServers, {
    immediate: true,
    onSuccess: (data) => console.log('Loaded:', data)
  });

  const { mutate, loading: saving } = useMutation(updateServer, {
    onSuccess: () => refetch()
  });

  return <div>{loading ? 'Loading...' : data?.length} servers</div>;
}
```

### useToast
Toast notification management.

```javascript
import { useToast } from './hooks';

function Component() {
  const { toasts, success, error, warning, info } = useToast();

  const handleClick = () => {
    success('Operation completed successfully!');
    error('Something went wrong!', 5000);
  };

  return <div>{toasts.map(toast => <Toast key={toast.id} {...toast} />)}</div>;
}
```

### usePagination
Pagination logic for lists and tables.

```javascript
import { usePagination } from './hooks';

function Component({ items }) {
  const {
    currentItems,
    currentPage,
    totalPages,
    nextPage,
    previousPage,
    goToPage
  } = usePagination(items, { initialPageSize: 25 });

  return (
    <div>
      {currentItems.map(item => <div key={item.id}>{item.name}</div>)}
      <button onClick={previousPage}>Previous</button>
      <button onClick={nextPage}>Next</button>
    </div>
  );
}
```

### useResponsive
Responsive breakpoint detection.

```javascript
import { useResponsive, useMediaQuery } from './hooks';

function Component() {
  const { isMobile, isTablet, isDesktop } = useResponsive();
  const isLargeScreen = useMediaQuery('lg');

  return (
    <div>
      {isMobile && <MobileView />}
      {isDesktop && <DesktopView />}
    </div>
  );
}
```

### useLocalStorage
localStorage persistence with JSON serialization.

```javascript
import { useLocalStorage } from './hooks';

function Component() {
  const [theme, setTheme, removeTheme] = useLocalStorage('theme', 'light');

  return (
    <button onClick={() => setTheme(theme === 'light' ? 'dark' : 'light')}>
      Current theme: {theme}
    </button>
  );
}
```

### useDebounce
Debounce values and callbacks.

```javascript
import { useDebounce, useDebouncedCallback } from './hooks';

function Component() {
  const [searchTerm, setSearchTerm] = useState('');
  const debouncedSearch = useDebounce(searchTerm, 500);

  const { callback: debouncedSave } = useDebouncedCallback((value) => {
    console.log('Saving:', value);
  }, 1000);

  useEffect(() => {
    // This only runs 500ms after user stops typing
    if (debouncedSearch) {
      performSearch(debouncedSearch);
    }
  }, [debouncedSearch]);

  return <input onChange={(e) => setSearchTerm(e.target.value)} />;
}
```

## Best Practices

1. **Cleanup**: All hooks properly clean up resources (timers, listeners, connections) in useEffect cleanup functions.

2. **SSR Compatibility**: Hooks check for `window` existence to work in server-side rendering environments.

3. **Error Handling**: Hooks handle edge cases like null values, network errors, and component unmounting.

4. **Type Safety**: All hooks include comprehensive JSDoc comments for type hints and documentation.

5. **Reusability**: Hooks are designed to be composable and reusable across different components.

## Testing

When testing components that use these hooks, consider:

- Mock WebSocket connections for `useWebSocket`
- Mock API calls for `useApi` and `useMutation`
- Use fake timers for `useDebounce` and `useThrottle`
- Mock localStorage for `useLocalStorage`

## Contributing

When adding new hooks:

1. Follow React hooks naming convention (`use[Name]`)
2. Add comprehensive JSDoc comments
3. Handle cleanup properly
4. Include edge case handling
5. Add examples to this README
6. Export from `index.js`
