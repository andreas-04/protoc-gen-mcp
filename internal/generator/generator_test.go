package generator

import (
	"strings"
	"testing"
)

func TestCamelToSnake(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"A", "a"},
		{"SayHello", "say_hello"},
		{"GreeterService", "greeter_service"},
		{"HTTPSServer", "https_server"},
		{"XMLParser", "xml_parser"},
		{"already_snake", "already_snake"},
		// camelToSnake only splits at lower→upper / upper-run→lower boundaries.
		// A digit→uppercase transition is intentionally not a split point.
		{"WithDigits1And2", "with_digits1and2"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := camelToSnake(tc.in); got != tc.want {
				t.Errorf("camelToSnake(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestToToolName(t *testing.T) {
	cases := []struct {
		svc, method, want string
	}{
		{"GreeterService", "SayHello", "greeter_service_say_hello"},
		{"Auth", "RefreshToken", "auth_refresh_token"},
		{"XMLParser", "ParseAll", "xml_parser_parse_all"},
	}
	for _, tc := range cases {
		t.Run(tc.svc+"."+tc.method, func(t *testing.T) {
			if got := toToolName(tc.svc, tc.method); got != tc.want {
				t.Errorf("toToolName(%q, %q) = %q; want %q", tc.svc, tc.method, got, tc.want)
			}
		})
	}
}

func TestIsOmitEmpty(t *testing.T) {
	cases := map[string]bool{
		// Scalars must NOT carry omitempty — false/0/"" are valid inputs we
		// need to forward to protojson.Unmarshal.
		"bool":    false,
		"int32":   false,
		"int64":   false,
		"uint32":  false,
		"uint64":  false,
		"float32": false,
		"float64": false,
		"string":  false,
		// Pointer wrappers, slices, and JSON values use omitempty so a missing
		// field stays nil instead of being marshalled as the type's zero value.
		"*bool":            true,
		"*string":          true,
		"*int32":           true,
		"[]string":         true,
		"[]int32":          true,
		"[]json.RawMessage": true,
		"json.RawMessage":  true,
	}
	for typ, want := range cases {
		t.Run(typ, func(t *testing.T) {
			if got := isOmitEmpty(typ); got != want {
				t.Errorf("isOmitEmpty(%q) = %v; want %v", typ, got, want)
			}
		})
	}
}

func TestSanitizeTag(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  hello  ", "hello"},
		{"line1\nline2", "line1 line2"},
		{"has \"quotes\"", "has 'quotes'"},
		{"has `backticks`", "has 'backticks'"},
		{"mixed `q1` and \"q2\"\nover two lines", "mixed 'q1' and 'q2' over two lines"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := sanitizeTag(tc.in); got != tc.want {
				t.Errorf("sanitizeTag(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeAlias(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "pb"},
		{"greeter", "greeter"},
		{"greeter_v1", "greeter_v1"},
		{"v1", "v1"}, // first char digit dropped, then "v1" remains... actually leading digit is dropped
		{"1invalid", "invalid"},
		{"with-dash", "withdash"},
		{"123", "pb"},
		{"my.pkg", "mypkg"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := sanitizeAlias(tc.in); got != tc.want {
				t.Errorf("sanitizeAlias(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestCommonPrefix(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{"identical", []string{"a", "b", "c"}, []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"shared prefix", []string{"a", "b", "c"}, []string{"a", "b", "d"}, []string{"a", "b"}},
		{"no overlap", []string{"a"}, []string{"b"}, []string{}},
		{"empty a", []string{}, []string{"x"}, []string{}},
		{"empty b", []string{"x"}, []string{}, []string{}},
		{"a shorter", []string{"a"}, []string{"a", "b"}, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commonPrefix(tc.a, tc.b)
			if !sliceEqual(got, tc.want) {
				t.Errorf("commonPrefix(%v, %v) = %v; want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestDeriveAggregatorPkg(t *testing.T) {
	cases := []struct {
		name    string
		pkgs    []string
		dir     string
		want    string
		wantErr bool
	}{
		{
			name: "single package, parent + dir",
			pkgs: []string{"github.com/acme/api/gen/greeter"},
			dir:  "mcpserver",
			want: "github.com/acme/api/gen/mcpserver",
		},
		{
			name: "two packages with common parent",
			pkgs: []string{
				"github.com/acme/api/gen/greeter",
				"github.com/acme/api/gen/billing",
			},
			dir:  "mcpserver",
			want: "github.com/acme/api/gen/mcpserver",
		},
		{
			name: "three packages with deeper common prefix",
			pkgs: []string{
				"github.com/acme/svc/auth/v1",
				"github.com/acme/svc/auth/v2",
				"github.com/acme/svc/auth/admin/v1",
			},
			dir:  "mcp",
			want: "github.com/acme/svc/auth/mcp",
		},
		{
			name:    "no packages",
			pkgs:    nil,
			dir:     "mcp",
			wantErr: true,
		},
		{
			name: "no common prefix",
			pkgs: []string{
				"github.com/foo/a",
				"gitlab.com/bar/b",
			},
			dir:     "mcp",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pkgs := make([]*registeredPackage, len(tc.pkgs))
			for i, p := range tc.pkgs {
				pkgs[i] = &registeredPackage{ImportPath: p}
			}
			got, err := deriveAggregatorPkg(pkgs, tc.dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("deriveAggregatorPkg(%v) = %q, nil; want error", tc.pkgs, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("deriveAggregatorPkg(%v): %v", tc.pkgs, err)
			}
			if got != tc.want {
				t.Errorf("deriveAggregatorPkg(%v) = %q; want %q", tc.pkgs, got, tc.want)
			}
		})
	}
}

func TestOptionsSet(t *testing.T) {
	t.Run("known keys", func(t *testing.T) {
		o := Default()
		pairs := [][2]string{
			{"grpc_package", "github.com/foo/bar"},
			{"gen_aggregator", "false"},
			{"aggregator_dir", "mcp"},
			{"aggregator_pkg", "github.com/foo/bar/mcp"},
			{"server_name", "acme-mcp"},
			{"server_version", "1.2.3"},
		}
		for _, p := range pairs {
			if err := o.Set(p[0], p[1]); err != nil {
				t.Fatalf("Set(%q, %q): %v", p[0], p[1], err)
			}
		}
		want := Options{
			GRPCPackage:   "github.com/foo/bar",
			GenAggregator: false,
			AggregatorDir: "mcp",
			AggregatorPkg: "github.com/foo/bar/mcp",
			ServerName:    "acme-mcp",
			ServerVersion: "1.2.3",
		}
		if o != want {
			t.Errorf("options after Set:\n got: %+v\nwant: %+v", o, want)
		}
	})

	t.Run("unknown key errors", func(t *testing.T) {
		o := Default()
		err := o.Set("unknown_opt", "x")
		if err == nil {
			t.Fatal("Set with unknown key returned nil; want error")
		}
		if !strings.Contains(err.Error(), "unknown_opt") {
			t.Errorf("error %q does not mention the unknown key", err)
		}
	})

	t.Run("invalid bool errors", func(t *testing.T) {
		o := Default()
		err := o.Set("gen_aggregator", "yes-please")
		if err == nil {
			t.Fatal("Set with non-bool returned nil; want error")
		}
	})

	t.Run("default initializes aggregator+version", func(t *testing.T) {
		o := Default()
		if !o.GenAggregator {
			t.Error("Default().GenAggregator = false; want true")
		}
		if o.ServerVersion == "" {
			t.Error("Default().ServerVersion is empty")
		}
	})
}

func TestImplsFieldDisambiguation(t *testing.T) {
	pkgs := []*registeredPackage{
		{Alias: "auth", Services: []registeredService{{GoName: "AuthService"}}},
		{Alias: "billing", Services: []registeredService{{GoName: "AuthService"}, {GoName: "BillingService"}}},
		{Alias: "admin", Services: []registeredService{{GoName: "AuthService"}}},
	}
	assignFieldNames(pkgs)

	want := []string{"AuthService", "BillingAuthService", "BillingService", "AdminAuthService"}
	var got []string
	for _, p := range pkgs {
		for _, s := range p.Services {
			got = append(got, s.FieldName)
		}
	}
	if !sliceEqual(got, want) {
		t.Errorf("field names = %v; want %v", got, want)
	}

	// All field names must be unique.
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Errorf("duplicate field name %q in %v", n, got)
		}
		seen[n] = true
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
