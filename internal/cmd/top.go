package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/spf13/cobra"
)

type ServerStats struct {
	Name       string
	Status     string
	CPU        float64
	Memory     float64
	MemoryUnit string
	Network    string
	Uptime     string
}

type topModel struct {
	cfg           *config.ComposeConfig
	runtime       container.Runtime
	stats         []ServerStats
	sortColumn    int
	sortAscending bool
	err           error
	quitting      bool
	lastUpdate    time.Time
}

type tickMsg time.Time

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			BorderBottom(true)

	cellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	selectedCellStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#7D56F4")).
				Foreground(lipgloss.Color("#FFFFFF"))

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF00")).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000"))

	statusPausedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFF00"))

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginTop(1)
)

func NewTopCommand() *cobra.Command {
	var refreshInterval int

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Real-time resource usage monitoring",
		Long: `Monitor CPU, memory, and network usage for all MCP servers in real-time.
Similar to 'docker stats' or 'top' command with sortable columns.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			return runTopMonitor(file, refreshInterval)
		},
	}

	cmd.Flags().IntVarP(&refreshInterval, "interval", "i", 1, "Refresh interval in seconds")

	return cmd
}

func runTopMonitor(configFile string, interval int) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	m := topModel{
		cfg:        cfg,
		runtime:    runtime,
		sortColumn: 0,
	}

	if err := m.fetchStats(); err != nil {
		return fmt.Errorf("failed to fetch initial stats: %w", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run monitor: %w", err)
	}

	return nil
}

func (m topModel) Init() tea.Cmd {
	return tea.Batch(
		m.tick(),
		tea.EnterAltScreen,
	)
}

func (m topModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true

			return m, tea.Quit

		case "1":
			m.sortColumn = 0
			m.sortAscending = !m.sortAscending
			m.sortStats()

		case "2":
			m.sortColumn = 1
			m.sortAscending = !m.sortAscending
			m.sortStats()

		case "3":
			m.sortColumn = 2
			m.sortAscending = !m.sortAscending
			m.sortStats()

		case "4":
			m.sortColumn = 3
			m.sortAscending = !m.sortAscending
			m.sortStats()

		case "5":
			m.sortColumn = 4
			m.sortAscending = !m.sortAscending
			m.sortStats()

		case "r":
			if err := m.fetchStats(); err != nil {
				m.err = err
			}
		}

	case tickMsg:
		if err := m.fetchStats(); err != nil {
			m.err = err
		}
		m.lastUpdate = time.Time(msg)

		return m, m.tick()
	}

	return m, nil
}

func (m topModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder

	s.WriteString(headerStyle.Render("MCP-Compose Resource Monitor"))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		s.WriteString("\n")
	}

	headers := []string{"NAME", "STATUS", "CPU %", "MEMORY", "NETWORK", "UPTIME"}
	widths := []int{20, 12, 10, 15, 20, 15}

	s.WriteString(headerStyle.Render(m.formatRow(headers, widths)))
	s.WriteString("\n")

	for _, stat := range m.stats {
		status := stat.Status
		switch stat.Status {
		case "running":
			status = statusRunningStyle.Render("running")
		case "exited", "stopped":
			status = statusStoppedStyle.Render(stat.Status)
		case "paused":
			status = statusPausedStyle.Render("paused")
		}

		row := []string{
			stat.Name,
			status,
			fmt.Sprintf("%.2f%%", stat.CPU),
			fmt.Sprintf("%.2f %s", stat.Memory, stat.MemoryUnit),
			stat.Network,
			stat.Uptime,
		}

		s.WriteString(cellStyle.Render(m.formatRow(row, widths)))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(footerStyle.Render(
		fmt.Sprintf("Last update: %s | 1-5: sort column | r: refresh | q: quit",
			m.lastUpdate.Format("15:04:05")),
	))

	return s.String()
}

func (m *topModel) formatRow(cells []string, widths []int) string {
	var row strings.Builder

	for i, cell := range cells {
		width := widths[i]
		if len(cell) > width {
			cell = cell[:width-3] + "..."
		}

		padding := width - len(stripANSI(cell))
		row.WriteString(cell)
		row.WriteString(strings.Repeat(" ", padding))
		row.WriteString(" ")
	}

	return row.String()
}

func stripANSI(str string) string {
	result := ""
	inEscape := false

	for _, char := range str {
		if char == '\x1b' {
			inEscape = true

			continue
		}

		if inEscape {
			if char == 'm' {
				inEscape = false
			}

			continue
		}

		result += string(char)
	}

	return result
}

func (m *topModel) sortStats() {
	if len(m.stats) == 0 {
		return
	}

	sort.Slice(m.stats, func(i, j int) bool {
		var less bool

		switch m.sortColumn {
		case 0:
			less = m.stats[i].Name < m.stats[j].Name
		case 1:
			less = m.stats[i].Status < m.stats[j].Status
		case 2:
			less = m.stats[i].CPU < m.stats[j].CPU
		case 3:
			less = m.stats[i].Memory < m.stats[j].Memory
		case 4:
			less = m.stats[i].Network < m.stats[j].Network
		}

		if m.sortAscending {
			return less
		}

		return !less
	})
}

func (m *topModel) fetchStats() error {
	stats := []ServerStats{}

	for serverName := range m.cfg.Servers {
		containerName := fmt.Sprintf("mcp-compose-%s", serverName)

		status, err := m.runtime.GetContainerStatus(containerName)
		if err != nil {
			continue
		}

		stat := ServerStats{
			Name:       serverName,
			Status:     status,
			CPU:        0.0,
			Memory:     0.0,
			MemoryUnit: "MB",
			Network:    "0 B / 0 B",
			Uptime:     "-",
		}

		if status == "running" {
			if resourceStats, err := m.runtime.GetContainerStats(containerName); err == nil {
				stat.CPU = resourceStats.CPUUsage
				stat.Memory = float64(resourceStats.MemoryUsage) / (1024 * 1024)
				stat.MemoryUnit = "MB"

				if resourceStats.NetworkIO.RxBytes > 0 || resourceStats.NetworkIO.TxBytes > 0 {
					stat.Network = fmt.Sprintf("%s / %s",
						formatBytes(uint64(resourceStats.NetworkIO.RxBytes)),
						formatBytes(uint64(resourceStats.NetworkIO.TxBytes)))
				}
			}

			if info, err := m.runtime.GetContainerInfo(containerName); err == nil {
				uptime := parseUptime(info.Status)
				stat.Uptime = uptime
			}
		}

		stats = append(stats, stat)
	}

	specialContainers := []string{
		"mcp-compose-http-proxy",
		"mcp-compose-dashboard",
		"mcp-compose-memory",
		"mcp-compose-task-scheduler",
		"mcp-compose-postgres-memory",
	}

	for _, containerName := range specialContainers {
		status, err := m.runtime.GetContainerStatus(containerName)
		if err != nil || status == "" {
			continue
		}

		displayName := strings.TrimPrefix(containerName, "mcp-compose-")

		stat := ServerStats{
			Name:       displayName,
			Status:     status,
			CPU:        0.0,
			Memory:     0.0,
			MemoryUnit: "MB",
			Network:    "0 B / 0 B",
			Uptime:     "-",
		}

		if status == "running" {
			if resourceStats, err := m.runtime.GetContainerStats(containerName); err == nil {
				stat.CPU = resourceStats.CPUUsage
				stat.Memory = float64(resourceStats.MemoryUsage) / (1024 * 1024)
				stat.MemoryUnit = "MB"

				if resourceStats.NetworkIO.RxBytes > 0 || resourceStats.NetworkIO.TxBytes > 0 {
					stat.Network = fmt.Sprintf("%s / %s",
						formatBytes(uint64(resourceStats.NetworkIO.RxBytes)),
						formatBytes(uint64(resourceStats.NetworkIO.TxBytes)))
				}
			}

			if info, err := m.runtime.GetContainerInfo(containerName); err == nil {
				uptime := parseUptime(info.Status)
				stat.Uptime = uptime
			}
		}

		stats = append(stats, stat)
	}

	m.stats = stats
	m.sortStats()

	return nil
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func parseUptime(status string) string {
	if strings.Contains(status, "Up") {
		parts := strings.Split(status, "Up ")
		if len(parts) > 1 {
			uptime := strings.TrimSpace(parts[1])

			commaIdx := strings.Index(uptime, ",")
			if commaIdx > 0 {
				uptime = uptime[:commaIdx]
			}

			parenIdx := strings.Index(uptime, "(")
			if parenIdx > 0 {
				uptime = strings.TrimSpace(uptime[:parenIdx])
			}

			return uptime
		}
	}

	return "-"
}