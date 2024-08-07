package templates

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/azure/azure-dev/cli/azd/pkg/output"
)

// Absolute returns an absolute template path, given a possibly relative template path.
// An absolute path corresponds to a fully-qualified URI to a git repository.
//
// If the path is a valid Git URL (http, https, ssh, git, file), it is returned as-is.
// If the path is a relative or absolute file path, it is converted to a file:// URL.
// If the path is a repo name or owner/repo format, it is converted to a GitHub URL.

func Absolute(path string) (string, error) {
	path = strings.TrimRight(path, string(filepath.Separator))

	// If the path is already a recognized Git URL, return as-is.
	if isGitURL(path) {
		return path, nil
	}

	// Check if path is a relative or absolute file path
	if isRelativePath(path) || isAbsolutePath(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for template: %w", err)
		}

		// Ensure the path is in URL format
		if isWindowsPath(absPath) {
			absPath = "/" + filepath.ToSlash(absPath)
		} else {
			absPath = filepath.ToSlash(absPath)
		}

		return fmt.Sprintf("file://%s", absPath), nil
	}

	// Handle known GitHub path formats
	switch strings.Count(path, "/") {
	case 0:
		return fmt.Sprintf("https://github.com/Azure-Samples/%s", path), nil
	case 1:
		return fmt.Sprintf("https://github.com/%s", path), nil
	default:
		return "", fmt.Errorf(
			"template '%s' should either be <owner>/<repo> for GitHub repositories, "+
				"or <repo> for Azure-Samples GitHub repositories", path)
	}
}

// isGitURL determines if the given path is a valid Git URL by checking for common Git URL formats.
func isGitURL(path string) bool {
	parsedURL, err := url.Parse(path)
	if err != nil {
		return false
	}

	switch parsedURL.Scheme {
	case "http", "https", "ssh", "git", "file":
		return true
	default:
		return false
	}
}

// isRelativePath checks if a path is relative (starts with a '.').
func isRelativePath(path string) bool {
	return strings.HasPrefix(path, ".")
}

// isAbsolutePath checks if a path is an absolute path, either POSIX or Windows style.
func isAbsolutePath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	if matched, _ := regexp.MatchString(`^[a-zA-Z]:\\`, path); matched {
		return true
	}
	return false
}

// isWindowsPath checks if the given path is a Windows-style path with a drive letter.
func isWindowsPath(path string) bool {
	return regexp.MustCompile(`^[a-zA-Z]:`).MatchString(path)
}

// Hyperlink returns a hyperlink to the given template path.
// If the path cannot be resolved absolutely, it is returned as-is.
func Hyperlink(path string) string {
	url, err := Absolute(path)
	if err != nil {
		log.Printf("error: getting absolute url from template: %v", err)
		return path
	}
	return output.WithHyperlink(url, path)
}
