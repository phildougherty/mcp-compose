package cmd

var systemServices = map[string]string{
	"proxy":           "mcp-compose-http-proxy",
	"dashboard":       "mcp-compose-dashboard",
	"task-scheduler":  "mcp-compose-task-scheduler",
	"memory":          "mcp-compose-memory",
	"postgres-memory": "mcp-compose-postgres-memory",
}

func IsSystemService(name string) bool {
	_, ok := systemServices[name]

	return ok
}

func GetSystemServiceContainerName(name string) (string, bool) {
	containerName, ok := systemServices[name]

	return containerName, ok
}

func SplitSystemAndUserServices(services []string) (systemSvcs []string, userSvcs []string) {
	systemSvcs = make([]string, 0)
	userSvcs = make([]string, 0)

	for _, service := range services {
		if IsSystemService(service) {
			systemSvcs = append(systemSvcs, service)
		} else {
			userSvcs = append(userSvcs, service)
		}
	}

	return systemSvcs, userSvcs
}