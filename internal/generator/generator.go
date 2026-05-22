package generator

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/andreas-04/protoc-gen-mcp/internal/tmpl"
)

// FileData is the data passed to the per-file register template.
type FileData struct {
	// SourceFile is the proto source path, e.g. "api/greeter.proto".
	SourceFile string
	// PackageName is the Go package name of the generated pb files, e.g. "greeter".
	PackageName string
	// Services contains one entry per proto service in the file.
	Services []ServiceData
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
	OutputGoType    string // unqualified proto response type, e.g. "SayHelloResponse"
	InputStructName string // generated struct name, e.g. "SayHelloInput"
	InputFields     []FieldData
}

// FieldData holds template data for a single message field.
type FieldData struct {
	GoName      string
	JSONName    string
	GoType      string
	Description string
	// OmitEmpty is true only for pointer, slice, and map/json types where the
	// zero value (nil) is meaningfully distinct from an explicit empty value.
	// Plain scalar types (bool, int32, string, …) never get omitempty so that
	// an MCP client sending false or 0 is not silently dropped before reaching
	// protojson.Unmarshal.
	OmitEmpty bool
}

// registeredService captures one service inside a registeredPackage.
// GoName is the proto service's Go identifier (used for Register*Tools /
// New*Client / *Server type references). FieldName is the Impls struct
// field name, disambiguated when the same GoName appears in two packages.
type registeredService struct {
	GoName    string
	FieldName string
}

// registeredPackage captures the data the aggregator needs to wire up a
// single generated proto package: where to import it from, what alias to
// use, and which services to register.
type registeredPackage struct {
	ImportPath string
	Alias      string
	Services   []registeredService
}

// AggregatorData is the template data for the standalone aggregator file.
type AggregatorData struct {
	PkgName       string
	PkgImportPath string // full import path users will write in their main
	ServerName    string
	ServerVersion string
	Packages      []registeredPackage
	SourceFiles   []string // for the DO NOT EDIT header
}

// Generator drives a single protoc-gen-mcp invocation. It is created once
// per Run, fed every generated file via AddFile, and emits the aggregator
// in Finalize.
type Generator struct {
	opts Options

	// registered tracks per-package state so the aggregator can wire every
	// generated Register*Tools call.
	registered []*registeredPackage
	// pkgByPath dedups when two files share the same Go package (e.g. two
	// .proto files with the same go_package).
	pkgByPath map[string]*registeredPackage
	// firstProtoBasename is used to derive a default server name.
	firstProtoBasename string
	// sourceFiles is the list of proto paths fed to the aggregator header.
	sourceFiles []string
}

// New returns a Generator configured with opts. Defaults from Default() are
// applied for any zero-valued option.
func New(opts Options) *Generator {
	if opts.AggregatorDir == "" {
		opts.AggregatorDir = "mcpserver"
	}
	if opts.ServerVersion == "" {
		opts.ServerVersion = "0.1.0"
	}
	return &Generator{
		opts:      opts,
		pkgByPath: make(map[string]*registeredPackage),
	}
}

// AddFile emits the per-file _mcp.pb.go library and records the file's
// services so they can be wired into the aggregator in Finalize.
func (g *Generator) AddFile(gen *protogen.Plugin, f *protogen.File) error {
	data, err := buildFileData(f)
	if err != nil {
		return err
	}
	if len(data.Services) == 0 {
		return nil
	}

	registerSrc, err := tmpl.Execute("register.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("register.go.tmpl: %w", err)
	}
	regFile := gen.NewGeneratedFile(f.GeneratedFilenamePrefix+"_mcp.pb.go", f.GoImportPath)
	if _, err := regFile.Write([]byte(registerSrc)); err != nil {
		return fmt.Errorf("writing register file: %w", err)
	}

	// Track this package for the aggregator.
	importPath := string(f.GoImportPath)
	if g.opts.GRPCPackage != "" {
		importPath = g.opts.GRPCPackage
	}
	pkg, ok := g.pkgByPath[importPath]
	if !ok {
		pkg = &registeredPackage{
			ImportPath: importPath,
			Alias:      string(f.GoPackageName),
		}
		g.pkgByPath[importPath] = pkg
		g.registered = append(g.registered, pkg)
	}
	for _, svc := range data.Services {
		pkg.Services = append(pkg.Services, registeredService{GoName: svc.GoName})
	}

	g.sourceFiles = append(g.sourceFiles, f.Desc.Path())
	if g.firstProtoBasename == "" {
		g.firstProtoBasename = strings.TrimSuffix(path.Base(f.Desc.Path()), ".proto")
	}
	return nil
}

// Finalize emits the aggregator file once all per-file libraries are in.
// It is a no-op when Options.GenAggregator is false or no services were
// registered.
func (g *Generator) Finalize(gen *protogen.Plugin) error {
	if !g.opts.GenAggregator || len(g.registered) == 0 {
		return nil
	}

	aggPkgPath := g.opts.AggregatorPkg
	if aggPkgPath == "" {
		var err error
		aggPkgPath, err = deriveAggregatorPkg(g.registered, g.opts.AggregatorDir)
		if err != nil {
			return fmt.Errorf("could not derive aggregator package path; set 'aggregator_pkg' opt: %w", err)
		}
	}
	pkgName := path.Base(aggPkgPath)

	// Resolve import aliases to avoid collisions when two packages share a
	// base name. Iterate in stable order so codegen output is deterministic.
	sort.SliceStable(g.registered, func(i, j int) bool {
		return g.registered[i].ImportPath < g.registered[j].ImportPath
	})
	used := map[string]bool{pkgName: true}
	for _, pkg := range g.registered {
		base := sanitizeAlias(pkg.Alias)
		alias := base
		for i := 2; used[alias]; i++ {
			alias = fmt.Sprintf("%s%d", base, i)
		}
		used[alias] = true
		pkg.Alias = alias
	}

	assignFieldNames(g.registered)

	serverName := g.opts.ServerName
	if serverName == "" {
		serverName = g.firstProtoBasename + "-mcp"
	}

	data := AggregatorData{
		PkgName:       pkgName,
		PkgImportPath: aggPkgPath,
		ServerName:    serverName,
		ServerVersion: g.opts.ServerVersion,
		Packages:      derefPackages(g.registered),
		SourceFiles:   g.sourceFiles,
	}
	src, err := tmpl.Execute("aggregator.go.tmpl", data)
	if err != nil {
		return fmt.Errorf("aggregator.go.tmpl: %w", err)
	}

	// Filename uses pkgName (last segment of the import path), not the raw
	// aggregator_dir opt. Otherwise a user who sets aggregator_pkg with a
	// different last segment would get a file path that disagrees with the
	// package declaration and can't be imported.
	filename := path.Join(pkgName, "mcpserver.go")
	out := gen.NewGeneratedFile(filename, protogen.GoImportPath(aggPkgPath))
	if _, err := out.Write([]byte(src)); err != nil {
		return fmt.Errorf("writing aggregator: %w", err)
	}
	return nil
}

// derefPackages converts []*registeredPackage to []registeredPackage so it can
// be passed to a text/template without callers depending on the pointer type.
func derefPackages(in []*registeredPackage) []registeredPackage {
	out := make([]registeredPackage, len(in))
	for i, p := range in {
		out[i] = *p
	}
	return out
}

// deriveAggregatorPkg picks an aggregator import path from the longest
// common '/'-segment prefix of the registered proto packages, plus dir.
//
// Single-package case: drop the package's own last segment first so the
// aggregator lives as a sibling rather than nested inside the proto package.
func deriveAggregatorPkg(pkgs []*registeredPackage, dir string) (string, error) {
	if len(pkgs) == 0 {
		return "", fmt.Errorf("no proto packages were registered")
	}
	segments := make([][]string, len(pkgs))
	for i, p := range pkgs {
		segments[i] = strings.Split(p.ImportPath, "/")
	}
	common := segments[0]
	for _, s := range segments[1:] {
		common = commonPrefix(common, s)
	}
	if len(pkgs) == 1 && len(common) > 0 {
		common = common[:len(common)-1]
	}
	if len(common) == 0 {
		return "", fmt.Errorf("proto packages have no common path prefix")
	}
	return path.Join(append(common, dir)...), nil
}

func commonPrefix(a, b []string) []string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			break
		}
		out = append(out, a[i])
	}
	return out
}

// assignFieldNames populates each registeredService.FieldName. First
// occurrence of a GoName wins the unqualified name; subsequent collisions
// get the package alias (TitleCase'd) prepended, then numeric suffixes if
// still colliding.
func assignFieldNames(pkgs []*registeredPackage) {
	used := map[string]bool{}
	for _, pkg := range pkgs {
		for i := range pkg.Services {
			name := pkg.Services[i].GoName
			if !used[name] {
				pkg.Services[i].FieldName = name
				used[name] = true
				continue
			}
			prefixed := titleFirst(pkg.Alias) + name
			candidate := prefixed
			for n := 2; used[candidate]; n++ {
				candidate = fmt.Sprintf("%s%d", prefixed, n)
			}
			pkg.Services[i].FieldName = candidate
			used[candidate] = true
		}
	}
}

// titleFirst returns s with its first rune mapped to upper case. Used to
// build Impls field names on collision (lowercase package alias 'auth' +
// 'AuthService' would produce 'authAuthService' — not a valid exported
// Go identifier — so we capitalize first).
func titleFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// sanitizeAlias strips characters that are not valid in a Go import alias.
// Package names from go_package are usually clean, but defensive cleanup
// keeps templated output compilable. Digits are dropped while the output is
// empty so the result never starts with a digit.
func sanitizeAlias(s string) string {
	if s == "" {
		return "pb"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '_' || unicode.IsLetter(r):
			b.WriteRune(r)
		case unicode.IsDigit(r) && b.Len() > 0:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "pb"
	}
	return b.String()
}

// buildFileData constructs the FileData from a parsed proto file.
func buildFileData(f *protogen.File) (FileData, error) {
	data := FileData{
		SourceFile:  f.Desc.Path(),
		PackageName: string(f.GoPackageName),
	}

	for _, svc := range f.Services {
		svcData, err := buildServiceData(svc)
		if err != nil {
			return FileData{}, err
		}
		if len(svcData.Methods) == 0 {
			// Service has only streaming RPCs; skip it.
			continue
		}
		data.Services = append(data.Services, svcData)
	}
	return data, nil
}

// buildServiceData converts a protogen.Service into ServiceData.
func buildServiceData(svc *protogen.Service) (ServiceData, error) {
	sd := ServiceData{
		GoName:      svc.GoName,
		Description: cleanComment(svc.Comments.Leading),
	}
	if sd.Description == "" {
		sd.Description = svc.GoName + " gRPC service."
	}

	for _, m := range svc.Methods {
		// Skip all streaming RPCs – only unary methods become MCP tools.
		if m.Desc.IsStreamingClient() || m.Desc.IsStreamingServer() {
			continue
		}
		md, err := buildMethodData(svc, m)
		if err != nil {
			return ServiceData{}, err
		}
		sd.Methods = append(sd.Methods, md)
	}
	return sd, nil
}

// buildMethodData converts a protogen.Method into MethodData.
func buildMethodData(svc *protogen.Service, m *protogen.Method) (MethodData, error) {
	desc := cleanComment(m.Comments.Leading)
	if desc == "" {
		desc = "Calls the " + svc.GoName + " " + m.GoName + " RPC."
	}

	md := MethodData{
		GoName:          m.GoName,
		ToolName:        toToolName(svc.GoName, m.GoName),
		Description:     desc,
		InputGoType:     m.Input.GoIdent.GoName,
		OutputGoType:    m.Output.GoIdent.GoName,
		InputStructName: m.GoName + "Input",
	}

	for _, field := range m.Input.Fields {
		md.InputFields = append(md.InputFields, buildFieldData(field))
	}
	return md, nil
}

// buildFieldData converts a protogen.Field into FieldData.
func buildFieldData(field *protogen.Field) FieldData {
	desc := cleanComment(field.Comments.Leading)
	if desc == "" {
		desc = cleanComment(field.Comments.Trailing)
	}
	goType := protoFieldGoType(field.Desc)
	return FieldData{
		GoName:      field.GoName,
		JSONName:    string(field.Desc.JSONName()),
		GoType:      goType,
		Description: sanitizeTag(desc),
		OmitEmpty:   isOmitEmpty(goType),
	}
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

// isOmitEmpty reports whether json:",omitempty" should be applied to a field
// with the given Go type.  Only pointer types (*T), slices ([]T), and opaque
// JSON types (json.RawMessage) use omitempty — for these, nil/empty is
// a meaningful signal that the field was absent.  Plain value types (bool,
// int32, string, …) must NOT use omitempty because json.Marshal would silently
// drop an explicit false/0/"" before protojson.Unmarshal sees it.
func isOmitEmpty(goType string) bool {
	return strings.HasPrefix(goType, "*") ||
		strings.HasPrefix(goType, "[]") ||
		goType == "json.RawMessage"
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
