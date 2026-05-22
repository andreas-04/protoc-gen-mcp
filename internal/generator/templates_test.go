package generator

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/andreas-04/protoc-gen-mcp/internal/tmpl"
)

// updateGolden controls whether failing golden assertions overwrite the
// .golden file instead of failing. Toggle with -update or UPDATE_GOLDENS=1.
//
// Local workflow:
//
//	go test ./internal/generator/... -update      # rewrite goldens
//	UPDATE_GOLDENS=1 go test ./internal/generator/...
var updateGolden = flag.Bool("update", false, "overwrite golden files instead of comparing")

func goldenEnabled() bool { return *updateGolden || os.Getenv("UPDATE_GOLDENS") != "" }

// TestRegisterTemplate_Golden renders register.go.tmpl against a fixture
// FileData that exercises the interesting cases (scalar fields, wrapper
// types, repeated, json.RawMessage, descriptions) and compares the output
// byte-for-byte against testdata/golden/register.go.golden.
func TestRegisterTemplate_Golden(t *testing.T) {
	data := FileData{
		SourceFile:  "sample/v1/sample.proto",
		PackageName: "samplev1",
		Services: []ServiceData{{
			GoName:      "EchoService",
			Description: "EchoService echoes messages back to the caller.",
			Methods: []MethodData{
				{
					GoName:          "Echo",
					ToolName:        "echo_service_echo",
					Description:     "Echo returns its input verbatim.",
					InputGoType:     "EchoRequest",
					InputStructName: "EchoInput",
					InputFields: []FieldData{
						{
							GoName:      "Message",
							JSONName:    "message",
							GoType:      "string",
							Description: "message is the text to echo.",
						},
						{
							GoName:    "Count",
							JSONName:  "count",
							GoType:    "int32",
							OmitEmpty: false,
						},
						{
							GoName:    "Tags",
							JSONName:  "tags",
							GoType:    "[]string",
							OmitEmpty: true,
						},
						{
							GoName:    "Optional",
							JSONName:  "optional",
							GoType:    "*string",
							OmitEmpty: true,
						},
						{
							GoName:    "Metadata",
							JSONName:  "metadata",
							GoType:    "json.RawMessage",
							OmitEmpty: true,
						},
					},
				},
				{
					GoName:          "Reverse",
					ToolName:        "echo_service_reverse",
					Description:     "Reverse returns its input with characters reversed.",
					InputGoType:     "ReverseRequest",
					InputStructName: "ReverseInput",
					InputFields: []FieldData{{
						GoName:   "Text",
						JSONName: "text",
						GoType:   "string",
					}},
				},
			},
		}},
	}

	got, err := tmpl.Execute("register.go.tmpl", data)
	if err != nil {
		t.Fatalf("Execute register.go.tmpl: %v", err)
	}
	assertGolden(t, "register.go.golden", got)
}

// TestAggregatorTemplate_Golden renders aggregator.go.tmpl against a
// fixture AggregatorData that wires two proto packages and compares the
// output byte-for-byte against testdata/golden/aggregator.go.golden.
func TestAggregatorTemplate_Golden(t *testing.T) {
	data := AggregatorData{
		PkgName:       "mcpserver",
		PkgImportPath: "github.com/acme/api/gen/mcpserver",
		ServerName:    "acme-mcp",
		ServerVersion: "1.2.3",
		Packages: []registeredPackage{
			{
				ImportPath: "github.com/acme/api/gen/auth",
				Alias:      "auth",
				Services: []registeredService{
					{GoName: "AuthService", FieldName: "AuthService"},
				},
			},
			{
				ImportPath: "github.com/acme/api/gen/billing",
				Alias:      "billing",
				Services: []registeredService{
					{GoName: "BillingService", FieldName: "BillingService"},
					{GoName: "InvoiceService", FieldName: "InvoiceService"},
				},
			},
		},
		SourceFiles: []string{"auth/v1/auth.proto", "billing/v1/billing.proto"},
	}

	got, err := tmpl.Execute("aggregator.go.tmpl", data)
	if err != nil {
		t.Fatalf("Execute aggregator.go.tmpl: %v", err)
	}
	assertGolden(t, "aggregator.go.golden", got)
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)

	if goldenEnabled() {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (run with -update to create): %v", path, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s\n\nRun with -update to refresh.", name, got, want)
	}
}
