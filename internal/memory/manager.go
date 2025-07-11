// internal/memory/manager.go
package memory

import (
	"fmt"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"os"
	"os/exec"
	"strings"
)

type Manager struct {
	cfg        *config.ComposeConfig
	runtime    container.Runtime
	configFile string
}

func NewManager(cfg *config.ComposeConfig, runtime container.Runtime) *Manager {

	return &Manager{
		cfg:     cfg,
		runtime: runtime,
	}
}

func (m *Manager) SetConfigFile(configFile string) {
	m.configFile = configFile
}

func (m *Manager) Start() error {
	fmt.Println("Starting MCP memory server...")

	// Get PostgreSQL password from config or environment first
	pgPassword := "password"
	if m.cfg.Memory.PostgresPassword != "" {
		pgPassword = m.cfg.Memory.PostgresPassword
	}
	// Also check environment variable directly
	if envPassword := os.Getenv("POSTGRES_PASSWORD"); envPassword != "" {
		pgPassword = envPassword
	}

	// Check if postgres-memory is running first
	postgresStatus, err := m.runtime.GetContainerStatus("mcp-compose-postgres-memory")
	if err != nil || postgresStatus != "running" {
		if err := m.startPostgres(pgPassword); err != nil {

			return fmt.Errorf("failed to start postgres-memory: %w", err)
		}
	}

	// Build memory server image
	if err := m.buildMemoryImage(); err != nil {

		return fmt.Errorf("failed to build memory image: %w", err)
	}

	// Stop existing container
	_ = m.runtime.StopContainer("mcp-compose-memory")

	// Ensure network exists
	networkExists, _ := m.runtime.NetworkExists("mcp-net")
	if !networkExists {
		if err := m.runtime.CreateNetwork("mcp-net"); err != nil {

			return fmt.Errorf("failed to create mcp-net network: %w", err)
		}
		fmt.Println("Created mcp-net network for memory server.")
	}

	// Get configuration values with defaults
	dbURL := fmt.Sprintf("postgresql://postgres:%s@mcp-compose-postgres-memory:5432/memory_graph?sslmode=disable", pgPassword)
	if m.cfg.Memory.DatabaseURL != "" {
		dbURL = m.cfg.Memory.DatabaseURL
		// Ensure sslmode=disable is included if not present
		if !strings.Contains(dbURL, "sslmode=") {
			if strings.Contains(dbURL, "?") {
				dbURL += "&sslmode=disable"
			} else {
				dbURL += "?sslmode=disable"
			}
		}
	}

	// Get CPU and memory limits with defaults
	cpus := "1.0"
	if m.cfg.Memory.CPUs != "" {
		cpus = m.cfg.Memory.CPUs
	}

	memory := "1g"
	if m.cfg.Memory.Memory != "" {
		memory = m.cfg.Memory.Memory
	}

	// Start memory server
	opts := &container.ContainerOptions{
		Name:     "mcp-compose-memory",
		Image:    "mcp-compose-memory:latest",
		Ports:    []string{"3001:3001"},
		Networks: []string{"mcp-net"},
		Env: map[string]string{
			"NODE_ENV":          "production",
			"DATABASE_URL":      dbURL,
			"POSTGRES_PASSWORD": pgPassword,
		},
		User:        "root",
		CPUs:        cpus,
		Memory:      memory,
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "memory",
		},
		RestartPolicy: "unless-stopped",
	}

	containerID, err := m.runtime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start memory container: %w", err)
	}

	fmt.Printf("Memory server container started with ID: %s\n", containerID[:12])
	fmt.Printf("Memory server is running at http://localhost:3001\n")

	return nil
}

func (m *Manager) startPostgres(pgPassword string) error {
	fmt.Println("Starting postgres-memory database...")

	// Get postgres configuration with defaults
	pgCpus := "2.0"
	if m.cfg.Memory.PostgresCPUs != "" {
		pgCpus = m.cfg.Memory.PostgresCPUs
	}

	pgMemory := "2g"
	if m.cfg.Memory.PostgresMemory != "" {
		pgMemory = m.cfg.Memory.PostgresMemory
	}

	pgDB := "memory_graph"
	if m.cfg.Memory.PostgresDB != "" {
		pgDB = m.cfg.Memory.PostgresDB
	}

	pgUser := "postgres"
	if m.cfg.Memory.PostgresUser != "" {
		pgUser = m.cfg.Memory.PostgresUser
	}

	// Get volumes with defaults
	volumes := []string{"postgres-memory-data:/var/lib/postgresql/data"}
	if len(m.cfg.Memory.Volumes) > 0 {
		volumes = m.cfg.Memory.Volumes
	}

	opts := &container.ContainerOptions{
		Name:     "mcp-compose-postgres-memory",
		Image:    "postgres:15-alpine",
		Networks: []string{"mcp-net"},
		Env: map[string]string{
			"POSTGRES_DB":       pgDB,
			"POSTGRES_USER":     pgUser,
			"POSTGRES_PASSWORD": pgPassword,
		},
		Volumes:     volumes,
		User:        "postgres",
		CPUs:        pgCpus,
		Memory:      pgMemory,
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "database",
		},
		RestartPolicy: "unless-stopped",
	}

	containerID, err := m.runtime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	fmt.Printf("Postgres-memory container started with ID: %s\n", containerID[:12])

	return nil
}

func (m *Manager) buildMemoryImage() error {
	fmt.Println("Building Go-based memory server image...")

	dockerfilePath := "dockerfiles/Dockerfile.memory-go"

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {

		return fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
	}

	fmt.Println("Building Go memory server image with fresh dependencies...")

	// Build with no cache to force fresh download of git repo
	buildCmd := exec.Command("docker", "build", "--no-cache", "-f", dockerfilePath, "-t", "mcp-compose-memory:latest", ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {

		return fmt.Errorf("failed to build Go memory image: %w", err)
	}

	fmt.Println("✅ Go memory server image built successfully")

	return nil
}

func (m *Manager) Stop() error {
	fmt.Println("Stopping MCP memory server...")

	if err := m.runtime.StopContainer("mcp-compose-memory"); err != nil {
		fmt.Printf("Warning: Failed to stop memory container: %v\n", err)
	}

	if err := m.runtime.StopContainer("mcp-compose-postgres-memory"); err != nil {
		fmt.Printf("Warning: Failed to stop postgres-memory container: %v\n", err)
	}

	fmt.Println("✅ Memory server stopped successfully.")

	return nil
}

func (m *Manager) Restart() error {
	fmt.Println("Restarting MCP memory server...")

	if err := m.Stop(); err != nil {
		fmt.Printf("Warning: Error during memory server shutdown: %v\n", err)
	}

	return m.Start()
}

func (m *Manager) Status() (string, error) {

	return m.runtime.GetContainerStatus("mcp-compose-memory")
}
