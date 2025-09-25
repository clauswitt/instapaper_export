package version

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// These will be set at build time via ldflags
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// GetVersion returns the version string with git information
func GetVersion() string {
	// If version was set at build time (release builds), use it
	if Version != "dev" {
		return Version
	}

	// Development build - get version from git
	return getVersionFromGit()
}

// GetFullVersion returns version with commit and build info
func GetFullVersion() string {
	version := GetVersion()

	if Commit != "unknown" && Date != "unknown" {
		return fmt.Sprintf("%s (commit %s, built %s)", version, Commit, Date)
	}

	return version
}

// getVersionFromGit builds version string from git state
func getVersionFromGit() string {
	// Get latest tag with v prefix
	latestTag := getLatestTag()
	if latestTag == "" {
		latestTag = "v0.0.0"
	}

	// Check if we're exactly on a tag
	currentCommit := getCurrentCommit()
	tagCommit := getTagCommit(latestTag)

	version := latestTag

	// If not on exact tag commit, add revision info
	if currentCommit != tagCommit {
		version += "-rev-" + currentCommit[:8]
	}

	// Check for uncommitted changes
	if hasUncommittedChanges() {
		version += "-unclean"
	}

	return version
}

// getLatestTag gets the latest git tag with v prefix
func getLatestTag() string {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0", "--match=v*")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getCurrentCommit gets the current commit hash
func getCurrentCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// getTagCommit gets the commit hash for a specific tag
func getTagCommit(tag string) string {
	cmd := exec.Command("git", "rev-list", "-n", "1", tag)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// hasUncommittedChanges checks if there are any uncommitted changes
func hasUncommittedChanges() bool {
	// Check for staged and unstaged changes
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Filter for Go files and related files
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return false
	}

	// Check if any changes affect Go files, go.mod, go.sum, or other build-relevant files
	relevantFilePattern := regexp.MustCompile(`\.(go|mod|sum)$|Dockerfile|Makefile`)

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		filename := strings.TrimSpace(line[3:])
		if relevantFilePattern.MatchString(filename) {
			return true
		}
	}

	return false
}

// GetMCPVersion returns version for MCP server
func GetMCPVersion() string {
	return GetVersion()
}