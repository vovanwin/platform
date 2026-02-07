package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vovanwin/platform/protogen/generator"
)

func main() {
	apiDir := flag.String("api", "./api", "путь к директории с .proto файлами")
	outputDir := flag.String("output", "./internal/controller", "путь к директории для генерации контроллеров")
	serverPkg := flag.String("server-pkg", "", "import path пакета server (по умолчанию: <module>/internal/pkg/server)")
	flag.Parse()

	root, err := os.Getwd()
	if err != nil {
		fatal("getwd: %v", err)
	}

	if err := generator.Run(root, *apiDir, *outputDir, *serverPkg); err != nil {
		fatal("%v", err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
