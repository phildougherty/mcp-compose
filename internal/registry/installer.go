package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/phildougherty/mcp-compose/internal/logging"
	"gopkg.in/yaml.v3"
)

type Installer struct {
	logger     *logging.Logger
	configPath string
}

type ServerConfig struct {
	Protocol string            `yaml:"protocol" json:"protocol"`
	Command  string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args     []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Volumes  []string          `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Image    string            `yaml:"image,omitempty" json:"image,omitempty"`
	URL      string            `yaml:"url,omitempty" json:"url,omitempty"`
}

func NewInstaller(configPath string, logger *logging.Logger) *Installer {
	return &Installer{
		logger:     logger,
		configPath: configPath,
	}
}

func (i *Installer) InstallServerToConfig(server *Server, userConfig map[string]interface{}) error {
	var templateConfig ServerConfig
	if err := json.Unmarshal(server.ConfigTemplate, &templateConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config template: %w", err)
	}

	if templateConfig.Protocol == "" {
		templateConfig.Protocol = server.Protocol
	}

	finalConfig := i.applyUserConfig(templateConfig, userConfig)
	finalConfig = i.expandVariables(finalConfig)

	if err := i.appendToYAML(server.Name, finalConfig); err != nil {
		return fmt.Errorf("failed to append to YAML: %w", err)
	}

	i.logger.Info("Successfully installed server '%s' to config file with protocol '%s'", server.Name, finalConfig.Protocol)

	return nil
}

func (i *Installer) applyUserConfig(template ServerConfig, userConfig map[string]interface{}) ServerConfig {
	config := template

	if env, ok := userConfig["env"].(map[string]interface{}); ok {
		if config.Env == nil {
			config.Env = make(map[string]string)
		}
		for k, v := range env {
			if str, ok := v.(string); ok {
				config.Env[k] = str
			}
		}
	}

	if volumes, ok := userConfig["volumes"].([]interface{}); ok {
		config.Volumes = []string{}
		for _, v := range volumes {
			if str, ok := v.(string); ok {
				config.Volumes = append(config.Volumes, str)
			}
		}
	}

	if command, ok := userConfig["command"].(string); ok {
		config.Command = command
	}

	if args, ok := userConfig["args"].([]interface{}); ok {
		config.Args = []string{}
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				config.Args = append(config.Args, str)
			}
		}
	}

	return config
}

func (i *Installer) expandVariables(config ServerConfig) ServerConfig {
	homeDir, _ := os.UserHomeDir()

	expandString := func(s string) string {
		s = strings.ReplaceAll(s, "${HOME}", homeDir)
		s = strings.ReplaceAll(s, "$HOME", homeDir)
		return os.ExpandEnv(s)
	}

	config.Command = expandString(config.Command)
	config.URL = expandString(config.URL)
	config.Image = expandString(config.Image)

	for idx := range config.Args {
		config.Args[idx] = expandString(config.Args[idx])
	}

	for key, val := range config.Env {
		config.Env[key] = expandString(val)
	}

	for idx := range config.Volumes {
		config.Volumes[idx] = expandString(config.Volumes[idx])
	}

	return config
}

func (i *Installer) appendToYAML(serverName string, config ServerConfig) error {
	if _, err := os.Stat(i.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", i.configPath)
	}

	data, err := os.ReadFile(i.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	servers, ok := yamlConfig["servers"].(map[string]interface{})
	if !ok {
		servers = make(map[string]interface{})
		yamlConfig["servers"] = servers
	}

	if _, exists := servers[serverName]; exists {
		return fmt.Errorf("server '%s' already exists in config", serverName)
	}

	serverYAML := make(map[string]interface{})
	serverYAML["protocol"] = config.Protocol

	if config.Command != "" {
		serverYAML["command"] = config.Command
	}
	if len(config.Args) > 0 {
		serverYAML["args"] = config.Args
	}
	if len(config.Env) > 0 {
		serverYAML["env"] = config.Env
	}
	if len(config.Volumes) > 0 {
		serverYAML["volumes"] = config.Volumes
	}
	if config.Image != "" {
		serverYAML["image"] = config.Image
	}
	if config.URL != "" {
		serverYAML["url"] = config.URL
	}

	servers[serverName] = serverYAML

	backupPath := i.configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		i.logger.Warning("Failed to create backup: %v", err)
	}

	newData, err := yaml.Marshal(yamlConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(i.configPath, newData, 0644); err != nil {
		if restoreErr := os.WriteFile(i.configPath, data, 0644); restoreErr != nil {
			i.logger.Error("Failed to restore backup after write error: %v", restoreErr)
		}
		return fmt.Errorf("failed to write config file: %w", err)
	}

	i.logger.Info("Created backup at: %s", backupPath)

	return nil
}

func (i *Installer) GetInstalledServerNames() ([]string, error) {
	if _, err := os.Stat(i.configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", i.configPath)
	}

	data, err := os.ReadFile(i.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	servers, ok := yamlConfig["servers"].(map[string]interface{})
	if !ok {
		return []string{}, nil
	}

	serverNames := make([]string, 0, len(servers))
	for name := range servers {
		serverNames = append(serverNames, name)
	}

	return serverNames, nil
}

func (i *Installer) UninstallServerFromConfig(serverName string) error {
	if _, err := os.Stat(i.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", i.configPath)
	}

	data, err := os.ReadFile(i.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	servers, ok := yamlConfig["servers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no servers section found in config")
	}

	if _, exists := servers[serverName]; !exists {
		return fmt.Errorf("server '%s' not found in config", serverName)
	}

	delete(servers, serverName)

	backupPath := i.configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		i.logger.Warning("Failed to create backup: %v", err)
	}

	newData, err := yaml.Marshal(yamlConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(i.configPath, newData, 0644); err != nil {
		if restoreErr := os.WriteFile(i.configPath, data, 0644); restoreErr != nil {
			i.logger.Error("Failed to restore backup after write error: %v", restoreErr)
		}
		return fmt.Errorf("failed to write config file: %w", err)
	}

	i.logger.Info("Removed server '%s' from config, backup at: %s", serverName, backupPath)

	return nil
}

func (i *Installer) IsServerInstalled(serverName string) (bool, error) {
	if _, err := os.Stat(i.configPath); os.IsNotExist(err) {
		return false, fmt.Errorf("config file not found: %s", i.configPath)
	}

	data, err := os.ReadFile(i.configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return false, fmt.Errorf("failed to parse YAML: %w", err)
	}

	servers, ok := yamlConfig["servers"].(map[string]interface{})
	if !ok {
		return false, nil
	}

	_, exists := servers[serverName]

	return exists, nil
}

func (i *Installer) GetConfigPath() string {
	return i.configPath
}

func (i *Installer) SetConfigPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	i.configPath = absPath

	return nil
}
