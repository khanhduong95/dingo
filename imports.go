package main

import (
	"strings"
)

// ImportMap stores the mapping from full package paths to their resolved short names
type ImportMap map[string]string

// CollectAndResolveImports gathers all imports from services and resolves conflicts
func CollectAndResolveImports(services Services) ImportMap {
	// First pass: collect all imports with their desired short names
	importsByShortName := make(map[string][]string) // shortName -> []fullPackagePath

	for _, serviceName := range services.ServiceNames() {
		definition := services[serviceName]
		imports := definition.Imports()

		for packagePath, shortName := range imports {
			if packagePath == "" {
				continue
			}

			// Use default short name if not specified
			if shortName == "" {
				ty := Type(packagePath)
				shortName = ty.LocalPackageName()
			}

			importsByShortName[shortName] = append(importsByShortName[shortName], packagePath)
		}
	}

	// Second pass: resolve conflicts
	resolved := make(ImportMap)

	for shortName, packagePaths := range importsByShortName {
		// Remove duplicates
		uniquePaths := make(map[string]bool)
		var paths []string
		for _, path := range packagePaths {
			if !uniquePaths[path] {
				uniquePaths[path] = true
				paths = append(paths, path)
			}
		}

		if len(paths) == 1 {
			// No conflict, use the original short name
			resolved[paths[0]] = shortName
		} else {
			// Conflict detected, resolve by adding more path segments
			resolved = resolveConflict(paths, shortName, resolved)
		}
	}

	return resolved
}

// resolveConflict generates unique names for conflicting packages
func resolveConflict(packagePaths []string, baseShortName string, resolved ImportMap) ImportMap {
	// Try to find a unique prefix for each conflicting package
	for _, path := range packagePaths {
		uniqueName := generateUniqueName(path, baseShortName, packagePaths)
		resolved[path] = uniqueName
	}

	return resolved
}

// generateUniqueName creates a unique package name by incorporating parent path segments
func generateUniqueName(packagePath string, baseShortName string, allPaths []string) string {
	parts := strings.Split(packagePath, "/")
	if len(parts) == 0 {
		return baseShortName
	}

	// Start from the parent directory and work backwards
	for i := len(parts) - 2; i >= 0; i-- {
		// Build name from current parent + base name
		parentPart := sanitizePackageName(parts[i])
		candidateName := parentPart + "_" + baseShortName

		// Check if this name is unique among all conflicting paths
		if isUniqueAmong(candidateName, packagePath, allPaths) {
			return candidateName
		}

		// If still not unique, try adding more parent segments
		if i > 0 {
			grandParent := sanitizePackageName(parts[i-1])
			candidateName = grandParent + "_" + parentPart + "_" + baseShortName
			if isUniqueAmong(candidateName, packagePath, allPaths) {
				return candidateName
			}
		}
	}

	// Fallback: use sanitized full path
	return sanitizePackageName(strings.ReplaceAll(packagePath, "/", "_"))
}

// isUniqueAmong checks if the candidate name would be unique for this package path
func isUniqueAmong(candidateName string, targetPath string, allPaths []string) bool {
	for _, path := range allPaths {
		if path == targetPath {
			continue
		}

		// Check if another path would also generate the same candidate name
		parts := strings.Split(path, "/")
		if len(parts) == 0 {
			continue
		}

		// Simple check: if the candidate contains a parent dir from target
		// it shouldn't match other paths with different parents
		targetParts := strings.Split(targetPath, "/")
		if len(targetParts) < 2 {
			continue
		}

		// Extract the distinguishing part
		targetParent := targetParts[len(targetParts)-2]
		pathParent := ""
		if len(parts) >= 2 {
			pathParent = parts[len(parts)-2]
		}

		// If parents are different, we're good
		if targetParent != pathParent && strings.Contains(candidateName, sanitizePackageName(targetParent)) {
			continue
		}

		// Parents are the same, so this name wouldn't be unique
		if targetParent == pathParent {
			return false
		}
	}

	return true
}

// sanitizePackageName converts a path segment into a valid Go identifier
func sanitizePackageName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")
	// Remove any other invalid characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	return name
}
