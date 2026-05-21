package generator

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/andreasneacsu/buf-gen-mcp/internal/tmpl"
)

// FileData is the data passed to both code-generation templates.
type FileData struct {
	// SourceFile is the proto source path, e.g. "api/greeter.proto".
	SourceFile string
	// PackageName is the Go package name of the generated pb files, e.g. "greeter".
	PackageName string
	// GRPCImportPath is the Go import path for the pb/gRPC package.
	GRPCImportPath string
	// ServerName is used as the MCP server name, e.g. "greeter-mcp".
	ServerName string
	// Services contains one entry per proto service in the file.
	Services []ServiceData
	// NeedsJSONImport is true when at least one field uses json.RawMessage.
	NeedsJSONImport bool
}

// ServiceData holds template data for a proto service.
type ServiceData struct {
	GoName      string
	Description string
	Methods     []MethodData
}

// MethodData holds template data for a single unary RPC.
type MethodData struct {
	GoName          string
	ToolName        string
	Description     string
	InputGoType     string // unqualified proto request type, e.g. "SayHelloRequest"
	InputStructName string // generated struct name, e.g. "SayHelloInput"
	InputFields     []FieldData
}

// FieldData holds template data for a single message field.
type FieldData struct {
	GoName      string
	JSONName    string
	GoType      string
	Description string
}

// GenerateFile produces MCP server code for all services in a single proto
// file.  When genServer is true it also emits the standalone cmd/mcp-server/main.go.
func GenerateFile(gen *protogen.Plugin, f *protogen.File, opts Options, genServer bool) error {
	data, err := buildFileData(f, opts)
	if err != nil {
		return err
	}

	// ── library file ──────────────────────────────────────────────────────
	registerSrc, err := tmpl.Execute("register.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("register.go.tmpl: %w", err)
	}
	regFile := gen.NewGeneratedFile(f.GeneratedFilenamePrefix+"_mcp.pb.go", f.GoImportPath)
	if _, err := regFile.Write([]byte(registerSrc)); err != nil {
		return fmt.Errorf("writing library file: %w", err)
	}

	// ── standalone server ─────────────────────────────────────────────────
	if genServer {
		serverSrc, err := tmpl.Execute("server.go.tmpl", data)
		if err != nil {
			return fmt.Errorf("server.go.tmpl: %w", err)
		}
		// Use "" as the import path: this is a main package.
		srvFile := gen.NewGeneratedFile("cmd/mcp-server/main.go", "")
		if _, err := srvFile.Write([]byte(serverSrc)); err != nil {
			return fmt.Errorf("writing server file: %w", err)
		}
	}

	return nil
}

// buildFileData constructs the FileData from a parsed proto file.
func buildFileData(f *protogen.File, opts Options) (FileData, error) {
	grpcImportPath := string(f.GoImportPath)
	if opts.GRPCPackage != "" {
		grpcImportPath = opts.GRPCPackage
	}

	// Derive a human-readable server name from the proto package name.
	pkgName := string(f.GoPackageName)
	serverName := pkgName
	if p := strings.TrimSuffix(path.Base(f.Desc.Path()), ".proto"); p != "" {
		serverName = p
	}

	data := FileData{
		SourceFile:     f.Desc.Path(),
		PackageName:    pkgName,
		GRPCImportPath: grpcImportPath,
		ServerName:     serverName + "-mcp",
	}

	for _, svc := range f.Services {
		svcData, hasJSON, err := buildServiceData(svc)
		if err != nil {
			return FileData{}, err
		}
		data.Services = append(data.Services, svcData)
		if hasJSON {
			data.NeedsJSONImport = true
		}
	}

	return data, nil
}

// buildServiceData converts a protogen.Service into ServiceData.
func buildServiceData(svc *protogen.Service) (ServiceData, bool, error) {
	sd := ServiceData{
		GoName:      svc.GoName,
		Description: cleanComment(svc.Comments.Leading),
	}
	if sd.Description == "" {
		sd.Description = svc.GoName + " gRPC service."
	}

	hasJSON := false
	for _, m := range svc.Methods {
		// Skip all streaming RPCs – only unary methods become MCP tools.
		if m.Desc.IsStreamingClient() || m.Desc.IsStreamingServer() {
			continue
		}
		md, jsonField, err := buildMethodData(svc, m)
		if err != nil {
			return ServiceData{}, false, err
		}
		sd.Methods = append(sd.Methods, md)
		if jsonField {
			hasJSON = true
		}
	}
	return sd, hasJSON, nil
}

// buildMethodData converts a protogen.Method into MethodData.
func buildMethodData(svc *protogen.Service, m *protogen.Method) (MethodData, bool, error) {
	desc := cleanComment(m.Comments.Leading)
	if desc == "" {
		desc = "Calls the " + svc.GoName + " " + m.GoName + " RPC."
	}

	md := MethodData{
		GoName:          m.GoName,
		ToolName:        toToolName(svc.GoName, m.GoName),
		Description:     desc,
		InputGoType:     m.Input.GoIdent.GoName,
		InputStructName: m.GoName + "Input",
	}

	hasJSON := false
	for _, field := range m.Input.Fields {
		fd, err := buildFieldData(field)
		if err != nil {
			return MethodData{}, false, err
		}
		md.InputFields = append(md.InputFields, fd)
		if needsJSONImport(fd.GoType) {
			hasJSON = true
		}
	}
	return md, hasJSON, nil
}

// buildFieldData converts a protogen.Field into FieldData.
func buildFieldData(field *protogen.Field) (FieldData, error) {
	desc := cleanComment(field.Comments.Leading)
	if desc == "" {
		desc = cleanComment(field.Comments.Trailing)
	}

	return FieldData{
		GoName:      field.GoName,
		JSONName:    string(field.Desc.JSONName()),
		GoType:      protoFieldGoType(field.Desc),
		Description: sanitizeTag(desc),
	}, nil
}

// cleanComment strips comment delimiters and trims whitespace from a
// protogen.Comments value.  Proto comments arrive without the "//" prefix.
func cleanComment(c protogen.Comments) string {
	return strings.TrimSpace(string(c))
}

// sanitizeTag makes a string safe to embed in a Go struct tag value.
// Backticks and double-quotes would break the tag literal, so we replace them.
func sanitizeTag(s string) string {
	s = strings.ReplaceAll(s, "`", "'")
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// toToolName converts a service name and method name into a snake_case MCP
// tool name.  Example: ("GreeterService", "SayHello") → "greeter_service_say_hello".
func toToolName(serviceName, methodName string) string {
	return camelToSnake(serviceName) + "_" + camelToSnake(methodName)
}

// camelToSnake converts a CamelCase identifier to snake_case.
func camelToSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			// Insert underscore when transitioning lower→upper or
			// at the start of a final uppercase run (e.g. "XMLParser" → "xml_parser").
			prev := runes[i-1]
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(prev) || (unicode.IsUpper(prev) && unicode.IsLower(next)) {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
