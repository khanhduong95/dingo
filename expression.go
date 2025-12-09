package main

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"github.com/elliotchance/pie/pie"
	"golang.org/x/tools/go/ast/astutil"
)

type Expression string

func (e Expression) DependencyNames() (deps []string) {
	for _, v := range regexp.MustCompile(`@{(.*?)}`).FindAllStringSubmatch(string(e), -1) {
		parts := strings.Split(v[1], "(")
		deps = append(deps, parts[0])
	}

	return pie.Strings(deps).Unique()
}

func (e Expression) Dependencies() (deps []string) {
	for _, v := range regexp.MustCompile(`@{(.*?)}`).FindAllStringSubmatch(string(e), -1) {
		deps = append(deps, v[1])
	}

	return pie.Strings(deps).Unique()
}

func (e Expression) performSubstitutions(file *File, services Services, fromArgs bool) string {
	stmt := string(e)

	// Replace environment variables.
	stmt = replaceAllStringSubmatchFunc(
		regexp.MustCompile(`\${(.*?)}`), stmt, func(i []string) string {
			astutil.AddImport(file.fset, file.file, "os")

			return fmt.Sprintf("os.Getenv(\"%s\")", i[1])
		})

	// Replace service names.
	stmt = replaceAllStringSubmatchFunc(
		regexp.MustCompile(`@{(.*?)}`), stmt, func(i []string) string {
			if fromArgs {
				return strings.Split(i[1], "(")[0]
			}

			if strings.Contains(i[1], "(") {
				return fmt.Sprintf("container.Get%s", i[1])
			}

			if _, existsService := services[i[1]]; !existsService {
				panic(fmt.Sprintf("service does not exist: %s", i[1]))
			}

			if _, ok := services[i[1]].ContainerFieldType(services).(*ast.FuncType); ok {
				return fmt.Sprintf("container.%s", i[1])
			}

			return fmt.Sprintf("container.Get%s()", i[1])
		})

	// Note: Package prefix replacement is now done with context in astFunctionBody
	// to avoid replacing the wrong package when there are conflicts

	return stmt
}

// replacePackagePrefixes replaces unqualified package names with their resolved aliases
// contextPackage is the full package path that this expression belongs to (from the service's type)
func (e Expression) replacePackagePrefixes(stmt string, importMap ImportMap) string {
	// Build a reverse map from short names to resolved names
	shortToResolved := make(map[string]string)

	for fullPath, resolvedName := range importMap {
		// Extract the default short name from the full path
		parts := strings.Split(fullPath, "/")
		if len(parts) > 0 {
			defaultShortName := parts[len(parts)-1]
			defaultShortName = strings.ReplaceAll(defaultShortName, "-", "_")

			// Only map if the resolved name is different from the default
			if defaultShortName != resolvedName {
				shortToResolved[defaultShortName] = resolvedName
			}
		}
	}

	// Replace package prefixes in the statement
	// Match pattern: packageName.Identifier
	for shortName, resolvedName := range shortToResolved {
		// Use word boundaries to avoid partial matches
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(shortName) + `\.`)
		stmt = pattern.ReplaceAllString(stmt, resolvedName+".")
	}

	return stmt
}

// replacePackagePrefixesWithContext replaces package prefixes with awareness of the service's context
func (e Expression) replacePackagePrefixesWithContext(stmt string, serviceType Type, importMap ImportMap) string {
	if serviceType.PackageName() == "" {
		return stmt
	}

	// Get the package path and default short name for this service's type
	servicePkgPath := serviceType.PackageName()
	parts := strings.Split(servicePkgPath, "/")
	if len(parts) == 0 {
		return stmt
	}

	defaultShortName := parts[len(parts)-1]
	defaultShortName = strings.ReplaceAll(defaultShortName, "-", "_")

	// Get the resolved name for this service's package
	if resolvedName, ok := importMap[servicePkgPath]; ok && resolvedName != defaultShortName {
		// Replace the default short name with the resolved name in this statement
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(defaultShortName) + `\.`)
		stmt = pattern.ReplaceAllString(stmt, resolvedName+".")
	}

	return stmt
}
