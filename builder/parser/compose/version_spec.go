package compose

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	composego "github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"
	libcomposeyaml "github.com/docker/libcompose/yaml"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/util"
)

// parseSpec parses Docker Compose Spec (no version or v3.8+) using compose-go v2
func parseSpec(bodys [][]byte) (ComposeObject, *FieldSupportReport, error) {
	report := NewFieldSupportReport()

	// Create a temporary directory to store compose files
	cacheName := uuid.New().String()
	cacheDir := "/cache/docker-compose/" + cacheName
	if err := util.CheckAndCreateDir(cacheDir); err != nil {
		return ComposeObject{}, report, fmt.Errorf("create cache docker compose file dir error: %s", err.Error())
	}

	// Write compose files to disk (compose-go requires file paths)
	var files []string
	for i, body := range bodys {
		filename := filepath.Join(cacheDir, fmt.Sprintf("%d-docker-compose.yml", i))
		if err := ioutil.WriteFile(filename, body, 0755); err != nil {
			return ComposeObject{}, report, fmt.Errorf("write cache docker compose file error: %s", err.Error())
		}
		files = append(files, filename)
	}

	// Load the compose project using compose-go
	ctx := context.Background()

	// compose-go requires a project name and we skip strict validation
	// to allow compose files that may have warnings but are still usable
	loadOptions := func(opts *composego.Options) {
		opts.SetProjectName("rainbond-app", true)
		// Skip consistency check to allow container_name with replicas
		opts.SkipConsistencyCheck = true
		// Skip validation to be more permissive
		opts.SkipValidation = true
	}

	project, err := composego.LoadWithContext(ctx, composetypes.ConfigDetails{
		WorkingDir:  cacheDir,
		ConfigFiles: toConfigFiles(files),
		Environment: buildEnvironmentMap(),
	}, loadOptions)
	if err != nil {
		return ComposeObject{}, report, fmt.Errorf("failed to load compose file: %s", err.Error())
	}

	// Convert compose-go project to ComposeObject
	co, conversionReport := composeGoToKomposeMapping(project)

	// Merge conversion report into main report
	if conversionReport != nil {
		report.Issues = append(report.Issues, conversionReport.Issues...)
	}

	return co, report, nil
}

// toConfigFiles converts file paths to ConfigFile structs
func toConfigFiles(files []string) []composetypes.ConfigFile {
	configFiles := make([]composetypes.ConfigFile, len(files))
	for i, file := range files {
		configFiles[i] = composetypes.ConfigFile{
			Filename: file,
		}
	}
	return configFiles
}

// buildEnvironmentMap builds environment variables map
func buildEnvironmentMap() map[string]string {
	env, err := buildEnvironment()
	if err != nil {
		logrus.Warnf("failed to build environment: %v", err)
		return make(map[string]string)
	}
	return env
}

// composeGoToKomposeMapping converts compose-go Project to ComposeObject
func composeGoToKomposeMapping(project *composetypes.Project) (ComposeObject, *FieldSupportReport) {
	report := NewFieldSupportReport()

	co := ComposeObject{
		ServiceConfigs: make(map[string]ServiceConfig),
	}

	// Convert each service
	for _, service := range project.Services {
		name := service.Name
		logrus.Debugf("compose spec service name is %s", name)

		serviceConfig := ServiceConfig{}
		serviceConfig.ContainerName = service.ContainerName
		if serviceConfig.ContainerName == "" {
			serviceConfig.ContainerName = name
		}
		serviceConfig.Image = service.Image
		serviceConfig.WorkingDir = service.WorkingDir
		serviceConfig.Annotations = service.Labels
		serviceConfig.CapAdd = service.CapAdd
		serviceConfig.CapDrop = service.CapDrop
		serviceConfig.Expose = service.Expose
		serviceConfig.Privileged = service.Privileged
		serviceConfig.User = service.User
		serviceConfig.Stdin = service.StdinOpen
		serviceConfig.Tty = service.Tty
		serviceConfig.TmpFs = service.Tmpfs

		// Command
		if service.Command != nil {
			serviceConfig.Command = service.Command
		}

		// Entrypoint
		if service.Entrypoint != nil {
			serviceConfig.Args = service.Entrypoint
		}

		// Environment variables
		serviceConfig.Environment = convertSpecEnvironment(service.Environment)

		// Ports
		serviceConfig.Port = convertSpecPorts(service.Ports)

		// Volumes
		serviceConfig.Volumes, serviceConfig.VolList = convertSpecVolumes(name, service.Volumes, report)

		// DependsOn - convert long format to simple array
		serviceConfig.DependsON = convertSpecDependsOn(name, service.DependsOn, report)

		// Restart policy
		if service.Restart != "" {
			serviceConfig.Restart = service.Restart
		}

		// Memory limits
		if service.MemLimit > 0 {
			serviceConfig.MemLimit = libcomposeyaml.MemStringorInt(service.MemLimit)
		}
		if service.MemReservation > 0 {
			serviceConfig.MemReservation = libcomposeyaml.MemStringorInt(service.MemReservation)
		}

		// CPU limits
		if service.CPUShares > 0 {
			serviceConfig.CPUShares = service.CPUShares
		}
		if service.CPUQuota > 0 {
			serviceConfig.CPUQuota = service.CPUQuota
		}

		// Build configuration
		if service.Build != nil {
			serviceConfig.Build = service.Build.Context
			serviceConfig.Dockerfile = service.Build.Dockerfile
			serviceConfig.BuildArgs = convertBuildArgs(service.Build.Args)
		}

		// Health check
		if service.HealthCheck != nil && !service.HealthCheck.Disable {
			serviceConfig.HealthChecks = convertSpecHealthCheck(service.HealthCheck)
		}

		// Deploy configuration
		if service.Deploy != nil {
			if service.Deploy.Replicas != nil {
				serviceConfig.Replicas = int(*service.Deploy.Replicas)
			}
			if service.Deploy.Resources.Limits != nil {
				if service.Deploy.Resources.Limits.NanoCPUs > 0 {
					cpuLimit := float64(service.Deploy.Resources.Limits.NanoCPUs)
					serviceConfig.CPULimit = int64(cpuLimit * 1000000000)
				}
				if service.Deploy.Resources.Limits.MemoryBytes > 0 {
					serviceConfig.MemLimit = libcomposeyaml.MemStringorInt(service.Deploy.Resources.Limits.MemoryBytes)
				}
			}
			if service.Deploy.Resources.Reservations != nil {
				if service.Deploy.Resources.Reservations.NanoCPUs > 0 {
					cpuReserv := float64(service.Deploy.Resources.Reservations.NanoCPUs)
					serviceConfig.CPUReservation = int64(cpuReserv * 1000000000)
				}
				if service.Deploy.Resources.Reservations.MemoryBytes > 0 {
					serviceConfig.MemReservation = libcomposeyaml.MemStringorInt(service.Deploy.Resources.Reservations.MemoryBytes)
				}
			}
			if service.Deploy.Placement.Constraints != nil {
				serviceConfig.Placement = make(map[string]string)
				for _, constraint := range service.Deploy.Placement.Constraints {
					parts := strings.SplitN(constraint, "==", 2)
					if len(parts) == 2 {
						serviceConfig.Placement[parts[0]] = parts[1]
					}
				}
			}
		}

		// Check for unsupported fields
		checkSpecUnsupportedFields(name, &service, report)

		co.ServiceConfigs[name] = serviceConfig
	}

	return co, report
}

// convertSpecEnvironment converts environment variables from compose-go format
func convertSpecEnvironment(env composetypes.MappingWithEquals) []EnvVar {
	var envVars []EnvVar
	for key, value := range env {
		if value == nil {
			envVars = append(envVars, EnvVar{Name: key, Value: ""})
		} else {
			envVars = append(envVars, EnvVar{Name: key, Value: *value})
		}
	}
	return envVars
}

// convertSpecPorts converts ports from compose-go format
func convertSpecPorts(ports []composetypes.ServicePortConfig) []Ports {
	var komposePorts []Ports
	for _, port := range ports {
		// Parse published port (can be a string like "8080" or empty)
		var hostPort int32
		if port.Published != "" {
			if p, err := strconv.ParseInt(port.Published, 10, 32); err == nil {
				hostPort = int32(p)
			}
		}

		komposePorts = append(komposePorts, Ports{
			HostPort:      hostPort,
			ContainerPort: int32(port.Target),
			HostIP:        port.HostIP,
			Protocol:      strings.ToUpper(port.Protocol),
		})
	}
	return komposePorts
}

// convertSpecVolumes converts volumes from compose-go format and detects config files
func convertSpecVolumes(serviceName string, volumes []composetypes.ServiceVolumeConfig, report *FieldSupportReport) ([]Volumes, []string) {
	var volArray []string
	var volStructs []Volumes

	for _, vol := range volumes {
		// Build volume string for VolList
		v := vol.Source
		if vol.Target != "" {
			v = v + ":" + vol.Target
		}
		if vol.ReadOnly {
			v = v + ":ro"
		}
		volArray = append(volArray, v)

		// Build volume struct
		volStruct := Volumes{
			SvcName:   serviceName,
			MountPath: vol.Target,
			Host:      vol.Source,
			Container: vol.Target,
			Mode:      "rw",
		}
		if vol.ReadOnly {
			volStruct.Mode = "ro"
		}

		// Check if this is a config file
		if isConfigFile(vol.Target, vol.Type) {
			volStruct.VolumeType = "config-file"
			report.AddInfo(serviceName, "volume",
				fmt.Sprintf("Detected config file: %s", vol.Target),
				"Will be created as Kubernetes ConfigMap")
		}

		volStructs = append(volStructs, volStruct)
	}

	return volStructs, volArray
}

// isConfigFile determines if a volume mount is a configuration file
func isConfigFile(mountPath string, volumeType string) bool {
	// Only bind mounts can be config files
	if volumeType != "bind" && volumeType != "" {
		return false
	}

	// Directories (ending with /) are not config files
	if strings.HasSuffix(mountPath, "/") {
		return false
	}

	// Check file extension
	configExtensions := []string{
		".conf", ".config", ".cfg", ".ini",
		".yaml", ".yml", ".json", ".xml", ".toml",
		".properties", ".env",
		".sh", ".bash", ".py", ".rb", ".js", ".ts",
	}
	for _, ext := range configExtensions {
		if strings.HasSuffix(strings.ToLower(mountPath), ext) {
			return true
		}
	}

	// Check special filenames
	basename := filepath.Base(mountPath)
	specialFiles := []string{
		"Dockerfile", "Makefile", "Rakefile", "Gemfile",
		".env", ".gitignore", ".dockerignore",
	}
	for _, special := range specialFiles {
		if basename == special {
			return true
		}
	}

	// Check path patterns (e.g., /etc/nginx/nginx.conf)
	pathPatterns := []string{"/etc/", "/config/", "/conf/"}
	for _, pattern := range pathPatterns {
		if strings.Contains(mountPath, pattern) && !strings.HasSuffix(mountPath, "/") {
			return true
		}
	}

	return false
}

// convertSpecDependsOn converts depends_on from compose-go format
func convertSpecDependsOn(serviceName string, dependsOn composetypes.DependsOnConfig, report *FieldSupportReport) []string {
	var deps []string
	hasConditions := false

	for dep := range dependsOn {
		deps = append(deps, dep)
	}

	// Check if any dependency has non-default conditions
	for _, config := range dependsOn {
		if config.Condition != "" && config.Condition != "service_started" {
			hasConditions = true
			break
		}
	}

	// Add one warning per service instead of one per dependency
	if hasConditions {
		report.AddDegraded(serviceName, "depends_on",
			"Has dependencies with health check conditions",
			"Health check conditions are not supported")
	}

	return deps
}

// convertBuildArgs converts build args from compose-go format
func convertBuildArgs(args composetypes.MappingWithEquals) map[string]*string {
	result := make(map[string]*string)
	for key, value := range args {
		result[key] = value
	}
	return result
}

// convertSpecHealthCheck converts health check from compose-go format
func convertSpecHealthCheck(hc *composetypes.HealthCheckConfig) HealthCheck {
	healthCheck := HealthCheck{
		Test:    hc.Test,
		Disable: hc.Disable,
	}

	if hc.Timeout != nil {
		healthCheck.Timeout = int32(time.Duration(*hc.Timeout).Seconds())
	}
	if hc.Interval != nil {
		healthCheck.Interval = int32(time.Duration(*hc.Interval).Seconds())
	}
	if hc.Retries != nil {
		healthCheck.Retries = int32(*hc.Retries)
	}
	if hc.StartPeriod != nil {
		healthCheck.StartPeriod = int32(time.Duration(*hc.StartPeriod).Seconds())
	}

	return healthCheck
}

// checkSpecUnsupportedFields checks for unsupported fields in compose spec
func checkSpecUnsupportedFields(serviceName string, service *composetypes.ServiceConfig, report *FieldSupportReport) {
	// Check for container_name with replicas (conflicting configuration)
	if service.ContainerName != "" && service.Deploy != nil && service.Deploy.Replicas != nil && *service.Deploy.Replicas > 1 {
		report.AddDegraded(serviceName, "container_name",
			fmt.Sprintf("container_name '%s' will be ignored when replicas > 1", service.ContainerName),
			"Rainbond will generate unique container names for each replica")
	}

	// Check for profiles (Compose Spec feature)
	if len(service.Profiles) > 0 {
		report.AddDegraded(serviceName, "profiles",
			fmt.Sprintf("Profiles %v will be ignored", service.Profiles),
			"Profiles are not supported in Rainbond, service will be deployed regardless of profile")
	}

	// Check for extends (Compose Spec feature - should be inlined by compose-go)
	if service.Extends != nil {
		report.AddInfo(serviceName, "extends",
			"Extends configuration has been automatically inlined",
			"No action needed, compose-go handles this automatically")
	}

	// Check for networks (limited support)
	if len(service.Networks) > 0 {
		report.AddDegraded(serviceName, "networks",
			"Custom networks configuration will be ignored",
			"Rainbond manages networking automatically")
	}

	// Check for secrets (not supported)
	if len(service.Secrets) > 0 {
		report.AddUnsupported(serviceName, "secrets",
			"Docker Compose secrets are not supported",
			"Use Rainbond's configuration management or environment variables instead")
	}

	// Check for configs (not supported)
	if len(service.Configs) > 0 {
		report.AddUnsupported(serviceName, "configs",
			"Docker Compose configs are not supported",
			"Use Rainbond's configuration file management instead")
	}

	// Check for external_links (not supported)
	if len(service.ExternalLinks) > 0 {
		report.AddUnsupported(serviceName, "external_links",
			"External links are not supported",
			"Use Rainbond's service discovery instead")
	}

	// Check for logging (limited support)
	if service.Logging != nil {
		report.AddDegraded(serviceName, "logging",
			"Custom logging configuration will be ignored",
			"Rainbond manages logging automatically")
	}

	// Check for ulimits (not supported)
	if service.Ulimits != nil && len(service.Ulimits) > 0 {
		report.AddUnsupported(serviceName, "ulimits",
			"Ulimits configuration is not supported",
			"Contact administrator to configure ulimits at node level")
	}
}

