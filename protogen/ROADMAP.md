# Protogen Roadmap

## Planned Features

### Validation
- **Proto validation** — check for required annotations (google.api.http) before generation

### Code Generation
- **Test generation** — create `_test.go` stubs for each method
- **Middleware/interceptors** — generate templates for logging, authorization, validation
- **Streaming support** — handle server/client/bidi streaming RPCs
- **DTO/mapping generation** — conversion between proto messages and domain models
- **Custom templates** — allow user-provided templates via CLI flag

### Developer Experience
- **Dry-run mode** — show what would be generated without writing files
- **Diff mode** — show differences between current and new files
- **Watch mode** — watch proto files for changes and auto-regenerate

### Documentation
- **Swagger/OpenAPI generation** — generate API specs from proto definitions
