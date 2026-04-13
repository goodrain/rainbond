package code

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	PythonStartSourceProcfile     = "procfile"
	PythonStartSourceAutoDetected = "auto-detected"
)

type pythonEntrypointCandidate struct {
	Module       string
	RelativePath string
	BaseName     string
	Content      string
	LowerContent string
	Depth        int
}

var pythonEntrypointBasePriority = []string{
	"main.py",
	"app.py",
	"server.py",
	"run.py",
	"application.py",
	"wsgi.py",
	"asgi.py",
	"api.py",
	"index.py",
	"__init__.py",
}

const pythonEntrypointSearchMaxDepth = 5

func DetectPythonPackageManager(buildPath string) string {
	for _, filename := range []string{"environment.yml", "environment.yaml", "conda.yml", "conda.yaml"} {
		if fileExists(filepath.Join(buildPath, filename)) {
			return "conda"
		}
	}
	if fileExists(filepath.Join(buildPath, "Pipfile")) {
		return "pipenv"
	}
	pyprojectPath := filepath.Join(buildPath, "pyproject.toml")
	if fileExists(pyprojectPath) {
		body, err := os.ReadFile(pyprojectPath)
		if err == nil && strings.Contains(string(body), "[tool.poetry]") {
			return "poetry"
		}
	}
	return "pip"
}

func DetectPythonStartCommand(buildPath, packageManager string) (string, string) {
	if ok, procfile := CheckProcfile(buildPath, Python); ok {
		return strings.TrimSpace(procfile), PythonStartSourceProcfile
	}

	if packageManager == "poetry" {
		if script := detectPoetryScriptName(buildPath); script != "" {
			return fmt.Sprintf("web: poetry run %s", script), PythonStartSourceAutoDetected
		}
	}

	if fileExists(filepath.Join(buildPath, "manage.py")) {
		return "web: python manage.py runserver 0.0.0.0:$_PORT", PythonStartSourceAutoDetected
	}

	if pythonManifestContainsDependency(buildPath, "pyramid") {
		if cmd := detectPyramidStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "aiohttp") {
		if cmd := detectAiohttpStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "quart") {
		if cmd := detectQuartStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "sanic") {
		if cmd := detectSanicStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "uvicorn") {
		if cmd := detectUvicornStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "gunicorn") {
		if cmd := detectGunicornStartCommand(buildPath); cmd != "" {
			return cmd, PythonStartSourceAutoDetected
		}
	}

	if pythonManifestContainsDependency(buildPath, "flask") {
		if module := detectFlaskAppModule(buildPath); module != "" {
			return fmt.Sprintf("web: flask --app %s run --host 0.0.0.0 --port $_PORT", module), PythonStartSourceAutoDetected
		}
	}

	return "", ""
}

func detectPyramidStartCommand(buildPath string) string {
	configFile := findPyramidConfigFile(buildPath)
	if configFile == "" {
		return ""
	}
	if portVariable := detectPyramidPortVariable(filepath.Join(buildPath, configFile)); portVariable != "" {
		return fmt.Sprintf("web: pserve %s %s=$_PORT", configFile, portVariable)
	}
	return fmt.Sprintf("web: pserve %s", configFile)
}

func detectAiohttpStartCommand(buildPath string) string {
	preferredFiles := []string{"server.py", "app.py", "main.py", "run.py", "application.py", "api.py"}
	if script := findPythonScriptByContent(buildPath, preferredFiles, []string{"aiohttp", "web.run_app("}); script != "" {
		return fmt.Sprintf("web: python %s", script)
	}
	if module, factory := findPythonModuleByFunction(buildPath, preferredFiles, []string{"init_func", "create_app", "init_app", "get_app", "make_app"}, []string{"aiohttp"}); module != "" {
		return fmt.Sprintf("web: python -m aiohttp.web -H 0.0.0.0 -P $_PORT %s:%s", module, factory)
	}
	return ""
}

func detectQuartStartCommand(buildPath string) string {
	preferredFiles := []string{"app.py", "main.py", "asgi.py", "application.py", "server.py", "run.py", "__init__.py"}
	if module, symbol := findPythonModuleByAssignment(buildPath, preferredFiles, []string{"app", "application"}, []string{"quart"}); module != "" {
		return fmt.Sprintf("web: hypercorn --bind 0.0.0.0:$_PORT %s:%s", module, symbol)
	}
	return ""
}

func detectSanicStartCommand(buildPath string) string {
	preferredFiles := []string{"server.py", "app.py", "main.py", "application.py", "run.py", "__init__.py"}
	if module, symbol := findPythonModuleByAssignment(buildPath, preferredFiles, []string{"app", "application"}, []string{"sanic"}); module != "" {
		return fmt.Sprintf("web: sanic %s:%s --host=0.0.0.0 --port=$_PORT", module, symbol)
	}
	if module, factory := findPythonModuleByFunction(buildPath, preferredFiles, []string{"create_app", "get_app", "make_app"}, []string{"sanic"}); module != "" {
		return fmt.Sprintf("web: sanic %s:%s --factory --host=0.0.0.0 --port=$_PORT", module, factory)
	}
	return ""
}

func detectUvicornStartCommand(buildPath string) string {
	preferredFiles := []string{"main.py", "app.py", "asgi.py", "application.py", "server.py", "run.py", "api.py", "__init__.py"}
	if module, symbol := findPythonModuleByAssignment(buildPath, preferredFiles, []string{"app", "application", "asgi_app"}, nil); module != "" {
		return fmt.Sprintf("web: uvicorn %s:%s --host 0.0.0.0 --port $_PORT", module, symbol)
	}
	return ""
}

func detectGunicornStartCommand(buildPath string) string {
	preferredFiles := []string{"wsgi.py", "app.py", "main.py", "application.py", "server.py", "run.py", "api.py", "__init__.py"}
	if module, symbol := findPythonModuleByAssignment(buildPath, preferredFiles, []string{"application", "app", "wsgi_app", "asgi_app"}, nil); module != "" {
		return fmt.Sprintf("web: gunicorn %s:%s --bind 0.0.0.0:$_PORT", module, symbol)
	}
	return ""
}

func detectFlaskAppModule(buildPath string) string {
	preferredFiles := []string{"app.py", "main.py", "server.py", "run.py", "application.py", "api.py", "__init__.py"}
	return findPythonModuleByContent(buildPath, preferredFiles, []string{"from flask import", "import flask", "flask(", "create_app", "make_app"})
}

func collectPythonEntrypointCandidates(buildPath string) []pythonEntrypointCandidate {
	var candidates []pythonEntrypointCandidate
	_ = filepath.WalkDir(buildPath, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		relPath, err := filepath.Rel(buildPath, filePath)
		if err != nil || relPath == "." {
			return nil
		}
		depth := strings.Count(relPath, string(filepath.Separator))
		if entry.IsDir() {
			if shouldSkipPythonSearchDir(entry.Name()) || depth > pythonEntrypointSearchMaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if depth > pythonEntrypointSearchMaxDepth || filepath.Ext(entry.Name()) != ".py" || !isPotentialPythonEntrypointFile(entry.Name()) {
			return nil
		}
		module := pythonModulePathFromFile(buildPath, filePath)
		if module == "" {
			return nil
		}
		body, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}
		content := string(body)
		candidates = append(candidates, pythonEntrypointCandidate{
			Module:       module,
			RelativePath: filepath.ToSlash(relPath),
			BaseName:     entry.Name(),
			Content:      content,
			LowerContent: strings.ToLower(content),
			Depth:        depth,
		})
		return nil
	})
	sort.SliceStable(candidates, func(i, j int) bool {
		leftRank := pythonEntrypointBaseRank(candidates[i].BaseName)
		rightRank := pythonEntrypointBaseRank(candidates[j].BaseName)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if candidates[i].Depth != candidates[j].Depth {
			return candidates[i].Depth < candidates[j].Depth
		}
		return candidates[i].RelativePath < candidates[j].RelativePath
	})
	return candidates
}

func findPythonModuleByAssignment(buildPath string, preferredFiles []string, symbols []string, needles []string) (string, string) {
	for _, candidate := range orderedPythonEntrypointCandidates(buildPath, preferredFiles) {
		if !containsAnySubstring(candidate.LowerContent, needles) {
			continue
		}
		for _, symbol := range symbols {
			if hasTopLevelAssignment(candidate.Content, symbol) {
				return candidate.Module, symbol
			}
		}
	}
	return "", ""
}

func findPythonModuleByFunction(buildPath string, preferredFiles []string, functions []string, needles []string) (string, string) {
	for _, candidate := range orderedPythonEntrypointCandidates(buildPath, preferredFiles) {
		if !containsAnySubstring(candidate.LowerContent, needles) {
			continue
		}
		for _, functionName := range functions {
			if hasTopLevelFunction(candidate.Content, functionName) {
				return candidate.Module, functionName
			}
		}
	}
	return "", ""
}

func findPythonModuleByContent(buildPath string, preferredFiles []string, needles []string) string {
	for _, candidate := range orderedPythonEntrypointCandidates(buildPath, preferredFiles) {
		if containsAnySubstring(candidate.LowerContent, needles) {
			return candidate.Module
		}
	}
	return ""
}

func findPythonScriptByContent(buildPath string, preferredFiles []string, needles []string) string {
	for _, candidate := range orderedPythonEntrypointCandidates(buildPath, preferredFiles) {
		if containsAnySubstring(candidate.LowerContent, needles) {
			return candidate.RelativePath
		}
	}
	return ""
}

func pythonModulePathFromFile(buildPath, filePath string) string {
	relPath, err := filepath.Rel(buildPath, filePath)
	if err != nil {
		return ""
	}
	modulePath := strings.TrimSuffix(relPath, ".py")
	modulePath = strings.TrimPrefix(modulePath, "src"+string(filepath.Separator))
	modulePath = strings.TrimSuffix(modulePath, string(filepath.Separator)+"__init__")
	modulePath = strings.Trim(modulePath, string(filepath.Separator))
	if modulePath == "" {
		return ""
	}
	return strings.ReplaceAll(modulePath, string(filepath.Separator), ".")
}

func findPyramidConfigFile(buildPath string) string {
	preferredFiles := []string{"development.ini", "production.ini"}
	for _, filename := range preferredFiles {
		if fileExists(filepath.Join(buildPath, filename)) {
			return filename
		}
	}
	files, err := filepath.Glob(filepath.Join(buildPath, "*.ini"))
	if err != nil {
		return ""
	}
	sort.Strings(files)
	for _, file := range files {
		baseName := filepath.Base(file)
		if baseName != "" {
			return baseName
		}
	}
	return ""
}

func detectPyramidPortVariable(configPath string) string {
	body, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	content := strings.ToLower(string(body))
	switch {
	case strings.Contains(content, "%(http_port)s"):
		return "http_port"
	case strings.Contains(content, "%(port)s"):
		return "port"
	default:
		return ""
	}
}

func matchesPreferredPythonFile(baseName string, preferredFiles []string) bool {
	if len(preferredFiles) == 0 {
		return true
	}
	for _, filename := range preferredFiles {
		if baseName == filename {
			return true
		}
	}
	return false
}

func orderedPythonEntrypointCandidates(buildPath string, preferredFiles []string) []pythonEntrypointCandidate {
	candidates := collectPythonEntrypointCandidates(buildPath)
	if len(preferredFiles) == 0 {
		return candidates
	}
	var ordered []pythonEntrypointCandidate
	for _, preferredFile := range preferredFiles {
		for _, candidate := range candidates {
			if candidate.BaseName == preferredFile {
				ordered = append(ordered, candidate)
			}
		}
	}
	return ordered
}

func isPotentialPythonEntrypointFile(name string) bool {
	return pythonEntrypointBaseRank(name) < len(pythonEntrypointBasePriority)+10
}

func pythonEntrypointBaseRank(name string) int {
	for idx, fileName := range pythonEntrypointBasePriority {
		if name == fileName {
			return idx
		}
	}
	return len(pythonEntrypointBasePriority) + 10
}

func containsAnySubstring(content string, needles []string) bool {
	if len(needles) == 0 {
		return true
	}
	for _, needle := range needles {
		if strings.Contains(content, strings.ToLower(strings.TrimSpace(needle))) {
			return true
		}
	}
	return false
}

func hasTopLevelAssignment(content string, name string) bool {
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `(?:\s*:\s*[^=]+)?\s*=`)
	return pattern.MatchString(content)
}

func hasTopLevelFunction(content string, name string) bool {
	pattern := regexp.MustCompile(`(?m)^(?:async\s+def|def)\s+` + regexp.QuoteMeta(name) + `\s*\(`)
	return pattern.MatchString(content)
}

func shouldSkipPythonSearchDir(name string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lowerName, ".") ||
		lowerName == "__pycache__" ||
		lowerName == "test" ||
		lowerName == "tests" ||
		lowerName == "dist" ||
		lowerName == "build" ||
		lowerName == "venv" ||
		lowerName == ".venv" ||
		lowerName == "node_modules" ||
		strings.HasSuffix(lowerName, ".egg-info")
}

func detectPoetryScriptName(buildPath string) string {
	pyprojectPath := filepath.Join(buildPath, "pyproject.toml")
	body, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return ""
	}
	content := string(body)
	blockIndex := strings.Index(content, "[tool.poetry.scripts]")
	if blockIndex == -1 {
		return ""
	}
	block := content[blockIndex:]
	lines := strings.Split(block, "\n")
	pattern := regexp.MustCompile(`^\s*([A-Za-z0-9._-]+)\s*=`)
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		matches := pattern.FindStringSubmatch(trimmed)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	return ""
}

func pythonManifestContainsDependency(buildPath string, dependency string) bool {
	dependency = strings.ToLower(strings.TrimSpace(dependency))
	for _, filename := range []string{"requirements.txt", "Pipfile", "pyproject.toml"} {
		body, err := os.ReadFile(filepath.Join(buildPath, filename))
		if err == nil && strings.Contains(strings.ToLower(string(body)), dependency) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
