# MCP Server Registry Dashboard Integration

This document describes the integration of the MCP Server Registry into the main Dashboard tab.

## Overview

The MCP Server Registry has been fully integrated into the Dashboard tab, providing a seamless experience for managing both running servers and browsing/installing new servers from the registry catalog. The separate Registry tab has been removed in favor of a unified view with a toggle between "My Servers" and "Browse Registry" modes.

## What Changed

### Frontend Changes

#### 1. Unified State Management (`store/dashboardStore.js`)

**Added State:**
- `viewMode`: Toggle between 'my-servers' and 'browse-registry'
- `registryServers`: Array of servers from the registry catalog
- `categories`: Registry categories for filtering
- `featuredServers`: Featured servers list
- `installedServers`: Servers installed from registry
- `selectedRegistryServer`: Currently selected registry server
- `categoryFilter`: Category filter for registry browsing
- `featuredOnly`: Toggle for showing only featured servers

**Added Actions:**
- `setViewMode(mode)`: Switch between views
- `fetchRegistryServers()`: Load registry catalog
- `fetchCategories()`: Load categories
- `fetchFeatured()`: Load featured servers
- `fetchInstalledServers()`: Load installed servers
- `fetchServerDetails(serverId)`: Get detailed server info
- `installServer(serverId, config)`: Install a server
- `uninstallServer(serverId)`: Uninstall a server
- `isServerInstalled(serverId)`: Check installation status
- `setCategoryFilter(category)`: Set category filter
- `setFeaturedOnly(featured)`: Toggle featured filter

#### 2. Unified Dashboard Component (`components/Dashboard/Dashboard.jsx`)

**New Features:**
- View mode toggle button (My Servers / Browse Registry)
- Conditional rendering based on viewMode
- Registry search and category filters in Browse Registry mode
- Registry server cards with install/uninstall actions
- Server details modal for registry servers
- Automatic synchronization between installed servers and running servers

**Layout:**
- **Header**: Title changes based on view mode with appropriate icon
- **View Toggle**: Segmented control to switch between modes
- **My Servers View**: Original dashboard with running server cards, metrics, and filters
- **Browse Registry View**: Grid of registry servers with search, category filters, featured section

#### 3. Removed Separate Registry Tab

**Modified Files:**
- `App.jsx`: Removed Registry tab from TABS array
- Removed Registry component import
- Removed RectangleStackIcon import (no longer needed for tab)

### Backend Changes

#### 1. New API Endpoint (`internal/dashboard/registry_handlers.go`)

**Added Endpoint:**
```
POST /api/registry/install-and-start
```

This endpoint combines server installation with automatic startup, providing a single action for users who want their server running immediately after installation.

**Functionality:**
1. Validates server exists in registry
2. Checks if server is already installed
3. Installs server to mcp-compose.yaml
4. Records installation in database
5. Returns success message indicating server will start automatically

### Documentation Changes

#### Updated REGISTRY.md

**Sections Modified:**
1. **Usage Guide**: Changed from "Click Registry tab" to "Click Browse Registry button in Dashboard"
2. **Architecture Overview**: Updated frontend section to reflect unified state management
3. **Features**: Added new integration features (Integrated Dashboard View, Unified State Management, etc.)
4. **API Documentation**: Added documentation for `/api/registry/install-and-start` endpoint
5. **Frontend Components**: Updated to describe unified Dashboard component and integrated store

## User Experience Flow

### Browsing the Registry

1. User opens Dashboard (default view: "My Servers")
2. User clicks "Browse Registry" in the view toggle
3. Dashboard switches to registry browsing mode
4. User can search, filter by category, or toggle featured-only
5. Featured servers and installed servers are displayed prominently

### Installing a Server

1. User clicks on a server card in Browse Registry mode
2. Server details modal opens showing:
   - Description and metadata
   - Configuration template
   - Required environment variables
   - Install/Uninstall button
3. User clicks "Install"
4. Server is added to mcp-compose.yaml
5. Installation is tracked in database
6. Modal closes and server appears in "Installed Servers" section
7. User can switch to "My Servers" to see the running server (if auto-started)

### Uninstalling a Server

1. User clicks on an installed server card
2. Server details modal shows "Uninstall" button
3. User confirms uninstallation
4. Server is removed from mcp-compose.yaml
5. Installation record is removed from database
6. Server is removed from "Installed Servers" section

## Files Modified

### Frontend
- `/internal/dashboard/frontend/src/store/dashboardStore.js` - Merged registry state
- `/internal/dashboard/frontend/src/components/Dashboard/Dashboard.jsx` - Complete rewrite with integration
- `/internal/dashboard/frontend/src/App.jsx` - Removed Registry tab

### Backend
- `/internal/dashboard/registry_handlers.go` - Added install-and-start endpoint

### Documentation
- `/REGISTRY.md` - Updated usage guide, architecture, and features
- `/REGISTRY_INTEGRATION.md` - This file (new)

## Files NOT Modified (Reused)

These existing registry components continue to work as-is:
- `/internal/dashboard/frontend/src/components/Registry/ServerCard.jsx` - Registry server display
- `/internal/dashboard/frontend/src/components/Registry/ServerDetails.jsx` - Server details modal
- `/internal/dashboard/frontend/src/components/Registry/CategoryFilter.jsx` - Category filtering
- `/internal/dashboard/frontend/src/components/Dashboard/ServerCard.jsx` - Running server cards
- All backend registry infrastructure (manager, installer, registry)

## State Management Integration

### Before (Separate Stores)
```
dashboardStore (running servers) ← Dashboard Component
registryStore (registry servers) ← Registry Component
```

### After (Unified Store)
```
dashboardStore (both running & registry) ← Dashboard Component
  ├─ My Servers view → Running servers
  └─ Browse Registry view → Registry servers
```

## Benefits of Integration

1. **Seamless Experience**: No navigation required, users can easily switch between managing running servers and browsing new ones
2. **Unified State**: Single source of truth for all server-related data
3. **Better Context**: Users can see what's running while browsing the registry
4. **Simplified Navigation**: One fewer tab in the main navigation
5. **Installation Feedback**: Immediate visibility when installed servers start running
6. **Reduced Code**: Eliminated duplicate state management logic

## Testing Checklist

To verify the integration works correctly:

### My Servers View
- [ ] Dashboard loads and shows running servers
- [ ] Server metrics display correctly
- [ ] Search and filters work
- [ ] Server cards expand/collapse
- [ ] Server actions work (restart, stop, logs)
- [ ] Auto-refresh functions correctly

### Browse Registry View
- [ ] View toggle switches to Browse Registry
- [ ] Registry servers load and display
- [ ] Search filters registry servers
- [ ] Category filters work
- [ ] Featured toggle works
- [ ] Installed servers section shows correctly
- [ ] Featured servers section displays

### Server Installation
- [ ] Clicking registry server opens details modal
- [ ] Install button adds server to config
- [ ] Success toast appears
- [ ] Server appears in Installed Servers section
- [ ] Switching to My Servers shows the running server (if auto-started)

### Server Uninstallation
- [ ] Uninstall button appears for installed servers
- [ ] Confirmation dialog appears
- [ ] Server is removed from config
- [ ] Server disappears from Installed Servers section
- [ ] Success toast appears

### State Synchronization
- [ ] Installing a server updates both registry and dashboard state
- [ ] Uninstalling a server updates both views
- [ ] Refreshing My Servers doesn't affect Browse Registry state
- [ ] Search query persists when switching views

## Migration Notes

### For Users
- The Registry tab is now integrated into the Dashboard tab
- Use the "Browse Registry" button in the Dashboard to access the registry
- All existing functionality remains the same

### For Developers
- The `registryStore` is now merged into `dashboardStore`
- Import registry functions from `dashboardStore` instead of `registryStore`
- The Registry component is no longer used as a standalone tab
- All registry components in `components/Registry/` are still used by Dashboard

## Future Enhancements

Possible future improvements to the integration:

1. **Smart Recommendations**: Show recommended registry servers based on current running servers
2. **Quick Actions**: Add "Install" button directly on registry server cards
3. **Update Notifications**: Show when installed servers have updates available
4. **Dependency Suggestions**: Suggest related servers when installing a server
5. **Start/Stop from Registry**: Allow starting/stopping installed servers from Browse Registry view
6. **Installation Queue**: Show pending installations when servers are being set up

## Troubleshooting

### Registry View Not Loading
- Check that PostgreSQL is configured and running
- Verify POSTGRES_URL environment variable is set
- Check browser console for API errors
- Verify registry API endpoints are registered

### View Toggle Not Working
- Clear browser cache and localStorage
- Check that viewMode state is being set correctly
- Verify React DevTools shows correct store state

### Installed Servers Not Showing
- Check that `/api/registry/installed` endpoint returns data
- Verify database has installation records
- Check that `fetchInstalledServers()` is being called on view change

### Search/Filters Not Working
- Verify searchQuery state is being set
- Check that categoryFilter is being passed to API
- Ensure `fetchRegistryServers()` is called when filters change

## Summary

The MCP Server Registry has been successfully integrated into the Dashboard tab, providing a unified experience for managing both running MCP servers and browsing/installing new servers from the registry catalog. The integration eliminates the need for a separate Registry tab while maintaining all existing functionality and improving the overall user experience.

**Key Achievement**: Users can now discover, install, and manage MCP servers all from a single, cohesive interface.
