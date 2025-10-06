# TUI Components

Terminal User Interface components for MCP-Compose, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## MCP Server Selector

The `mcp_selector.go` component provides an interactive UI for selecting which MCP servers should be enabled for a chat session.

### Features

- **Checkbox list** with server name, status, protocol, and tool count
- **Keyboard navigation** with vim-style (j/k) or arrow keys
- **Batch operations**: Select All (a) and Deselect All (d)
- **Status indicators**: Color-coded server status (Running/Stopped)
- **Live selection counter**: Shows number of selected servers
- **Styled** to match the existing TUI theme from `styles.go`

### Usage with ChatService

```go
import (
    "github.com/phildougherty/mcp-compose/internal/tui"
    "github.com/phildougherty/mcp-compose/internal/dashboard"
)

func configureMCPServers(chatService *dashboard.ChatService, sessionID string) error {
    // Get available servers from ChatService
    availableServers, err := chatService.GetAvailableMCPServers()
    if err != nil {
        return err
    }

    // Get currently enabled servers for this session
    currentlyEnabled, err := chatService.GetSessionMCPServers(sessionID)
    if err != nil {
        currentlyEnabled = []string{}
    }

    // Run the selector UI
    selectedServers, confirmed, err := tui.RunMCPServerSelector(availableServers, currentlyEnabled)
    if err != nil {
        return err
    }

    // If user confirmed (didn't cancel), update the session
    if confirmed {
        return chatService.SetSessionMCPServers(sessionID, selectedServers)
    }

    return nil
}
```

### Direct Model Usage

```go
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/phildougherty/mcp-compose/internal/tui"
)

func main() {
    servers := []tui.MCPServerInfo{
        {
            Name:      "filesystem",
            Status:    "Running",
            Protocol:  "stdio",
            ToolCount: 5,
            Enabled:   true,
        },
        {
            Name:      "brave-search",
            Status:    "Running",
            Protocol:  "stdio",
            ToolCount: 3,
            Enabled:   false,
        },
    }

    model := tui.NewMCPSelectorModel(servers)
    p := tea.NewProgram(model)

    finalModel, err := p.Run()
    if err != nil {
        panic(err)
    }

    if m, ok := finalModel.(tui.MCPSelectorModel); ok {
        if m.WasConfirmed() {
            selected := m.GetSelectedServers()
            // Use selected servers...
        }
    }
}
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Navigate up |
| `↓` / `j` | Navigate down |
| `space` | Toggle current server |
| `a` | Select all servers |
| `d` | Deselect all servers |
| `enter` | Confirm selection |
| `q` / `esc` / `ctrl+c` | Cancel |

### Data Structure

The `MCPServerInfo` struct holds the display information for each server:

```go
type MCPServerInfo struct {
    Name      string  // Server name (e.g., "filesystem")
    Status    string  // Server status ("Running", "Stopped", etc.)
    Protocol  string  // Protocol type ("stdio", "http", "sse", etc.)
    ToolCount int     // Number of tools available from this server
    Enabled   bool    // Whether this server is enabled for the session
}
```

### Integration Points

The MCP selector integrates with:

1. **ChatService.GetAvailableMCPServers()** - Retrieves list of running MCP servers with tool counts
2. **ChatService.GetSessionMCPServers(sessionID)** - Gets currently enabled servers for a session
3. **ChatService.SetSessionMCPServers(sessionID, servers)** - Updates enabled servers for a session

### Styling

All colors and styles are defined in `styles.go` to maintain consistency across the TUI:

- **Primary color**: Purple (`#8B5CF6`)
- **Highlight color**: Light Purple (`#A78BFA`)
- **Success color**: Green (`#10B981`)
- **Error color**: Red (`#EF4444`)
- **Warning color**: Yellow (`#F59E0B`)
- **Muted color**: Gray (`#6B7280`)
