package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func ConvertToMCPServerInfo(servers []map[string]interface{}, enabledServers []string) []MCPServerInfo {
	serverInfos := make([]MCPServerInfo, 0, len(servers))

	enabledMap := make(map[string]bool)
	for _, name := range enabledServers {
		enabledMap[name] = true
	}

	for _, server := range servers {
		name, _ := server["name"].(string)
		status, _ := server["status"].(string)
		protocol, _ := server["protocol"].(string)
		toolCount, _ := server["tool_count"].(int)

		if toolCount == 0 {
			if tools, ok := server["tools"].([]interface{}); ok {
				toolCount = len(tools)
			}
		}

		serverInfo := MCPServerInfo{
			Name:      name,
			Status:    status,
			Protocol:  protocol,
			ToolCount: toolCount,
			Enabled:   enabledMap[name],
		}

		serverInfos = append(serverInfos, serverInfo)
	}

	return serverInfos
}

func RunMCPServerSelector(availableServers []map[string]interface{}, currentlyEnabled []string) ([]string, bool, error) {
	servers := ConvertToMCPServerInfo(availableServers, currentlyEnabled)

	model := NewMCPSelectorModel(servers)

	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return nil, false, err
	}

	if m, ok := finalModel.(MCPSelectorModel); ok {
		if m.WasConfirmed() {
			return m.GetSelectedServers(), true, nil
		}

		return nil, false, nil
	}

	return nil, false, nil
}
