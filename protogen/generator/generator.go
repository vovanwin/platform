package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

type rpcMethod struct {
	Name     string
	Request  string
	Response string
}

type service struct {
	Name       string
	Methods    []rpcMethod
	GoPackage  string // full import path, e.g. "github.com/vovanwin/template/pkg/template"
	PbAlias    string // package alias, e.g. "template"
	StructName string // e.g. "TemplateGRPCServer"
	DirName    string // e.g. "template"
}

// Run запускает генерацию контроллеров.
// root — корень проекта (где лежит go.mod).
// apiDir — относительный или абсолютный путь к директории с proto файлами.
// outputDir — относительный или абсолютный путь для генерации контроллеров.
// serverPkg — import path пакета server (если пустой — вычисляется из go.mod).
func Run(root, apiDir, outputDir, serverPkg string) error {
	goModule := parseGoModule(filepath.Join(root, "go.mod"))
	if goModule == "" {
		return fmt.Errorf("could not parse module path from go.mod")
	}

	if serverPkg == "" {
		serverPkg = "github.com/vovanwin/platform/server"
	}

	absAPI := absPath(root, apiDir)
	absOutput := absPath(root, outputDir)

	protoFiles, err := filepath.Glob(filepath.Join(absAPI, "*", "*.proto"))
	if err != nil {
		return fmt.Errorf("glob: %w", err)
	}

	if len(protoFiles) == 0 {
		fmt.Println("no .proto files found in", absAPI)
		return nil
	}

	for _, pf := range protoFiles {
		services, err := parseProto(pf)
		if err != nil {
			return fmt.Errorf("parse %s: %w", pf, err)
		}

		for _, svc := range services {
			controllerDir := filepath.Join(absOutput, svc.DirName)
			if err := os.MkdirAll(controllerDir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", controllerDir, err)
			}

			// generate controller.go if missing
			if err := writeIfMissing(
				filepath.Join(controllerDir, "controller.go"),
				genController(svc),
			); err != nil {
				return err
			}

			// generate module.go if missing
			if err := writeIfMissing(
				filepath.Join(controllerDir, "module.go"),
				genModule(svc, goModule, serverPkg),
			); err != nil {
				return err
			}

			// generate method stubs for missing methods
			for _, m := range svc.Methods {
				fileName := CamelToSnake(m.Name) + ".go"
				if err := writeIfMissing(
					filepath.Join(controllerDir, fileName),
					genMethod(svc, m),
				); err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("done")
	return nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("created %s\n", path)
	return nil
}

func absPath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

var (
	reGoPackage = regexp.MustCompile(`option\s+go_package\s*=\s*"([^"]+)"`)
	reService   = regexp.MustCompile(`service\s+(\w+)\s*\{`)
	reRPC       = regexp.MustCompile(`rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)`)
	reModule    = regexp.MustCompile(`(?m)^module\s+(\S+)`)
)

func parseGoModule(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	m := reModule.FindStringSubmatch(string(data))
	if m == nil {
		return ""
	}
	return m[1]
}

func parseProto(path string) ([]service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	goPackage, pbAlias := parseGoPackage(content)
	if goPackage == "" {
		return nil, fmt.Errorf("go_package option not found in %s", path)
	}

	var services []service

	svcMatches := reService.FindAllStringIndex(content, -1)
	for i, loc := range svcMatches {
		svcName := reService.FindStringSubmatch(content[loc[0]:loc[1]])[1]

		start := loc[1]
		end := len(content)
		if i+1 < len(svcMatches) {
			end = svcMatches[i+1][0]
		}
		block := content[start:end]

		var methods []rpcMethod
		for _, m := range reRPC.FindAllStringSubmatch(block, -1) {
			methods = append(methods, rpcMethod{
				Name:     m[1],
				Request:  m[2],
				Response: m[3],
			})
		}

		dirName := strings.TrimSuffix(svcName, "Service")
		dirName = strings.ToLower(dirName)

		structName := strings.TrimSuffix(svcName, "Service") + "GRPCServer"

		services = append(services, service{
			Name:       svcName,
			Methods:    methods,
			GoPackage:  goPackage,
			PbAlias:    pbAlias,
			StructName: structName,
			DirName:    dirName,
		})
	}

	return services, nil
}

func parseGoPackage(content string) (goPackage, alias string) {
	m := reGoPackage.FindStringSubmatch(content)
	if m == nil {
		return "", ""
	}
	raw := m[1]
	if idx := strings.LastIndex(raw, ";"); idx != -1 {
		return raw[:idx], raw[idx+1:]
	}
	parts := strings.Split(raw, "/")
	return raw, parts[len(parts)-1]
}

// CamelToSnake converts CamelCase to snake_case.
func CamelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
