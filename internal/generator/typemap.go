package generator

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// wellKnownTypes maps the full name of well-known proto message types to the
// Go type used in generated MCP tool input structs.
var wellKnownTypes = map[string]string{
	"google.protobuf.Timestamp":   "string", // RFC 3339, e.g. "2006-01-02T15:04:05Z"
	"google.protobuf.Duration":    "string", // e.g. "10s"
	"google.protobuf.BoolValue":   "*bool",
	"google.protobuf.StringValue": "*string",
	"google.protobuf.Int32Value":  "*int32",
	"google.protobuf.Int64Value":  "*int64",
	"google.protobuf.UInt32Value": "*uint32",
	"google.protobuf.UInt64Value": "*uint64",
	"google.protobuf.FloatValue":  "*float32",
	"google.protobuf.DoubleValue": "*float64",
	"google.protobuf.BytesValue":  "*string",
	"google.protobuf.Struct":      "json.RawMessage",
	"google.protobuf.Value":       "json.RawMessage",
	"google.protobuf.ListValue":   "json.RawMessage",
	"google.protobuf.Any":         "json.RawMessage",
	"google.protobuf.FieldMask":   "string",
	"google.protobuf.Empty":       "struct{}",
}

// protoFieldGoType returns the Go type string for a proto field descriptor.
// Nested messages that are not well-known types are represented as
// json.RawMessage so that protojson can unmarshal them faithfully.
//
// proto3 'optional' scalar fields (or any field with explicit presence)
// are emitted as pointers so the MCP client can omit them and still let
// protojson observe absence — vs. an explicit zero value.
func protoFieldGoType(field protoreflect.FieldDescriptor) string {
	// Map fields → opaque JSON object
	if field.IsMap() {
		return "json.RawMessage"
	}

	base := scalarGoType(field)

	if field.IsList() {
		return "[]" + base
	}

	if isOptionalScalar(field, base) {
		return "*" + base
	}
	return base
}

// isOptionalScalar reports whether field has explicit presence for a plain
// scalar Go type — i.e. a proto3 'optional' keyword on bool/string/int/...
// The wrapper well-known types are already pointers via wellKnownTypes,
// and we never pointer-ify json.RawMessage or struct{}.
func isOptionalScalar(field protoreflect.FieldDescriptor, base string) bool {
	if !field.HasPresence() {
		return false
	}
	if base == "json.RawMessage" || base == "struct{}" {
		return false
	}
	// Already a pointer (well-known wrapper type) — leave it alone.
	if len(base) > 0 && base[0] == '*' {
		return false
	}
	return true
}

func scalarGoType(field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.EnumKind:
		return "string" // use string enum names for LLM friendliness
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BytesKind:
		// bytes fields are base64-encoded strings in JSON
		return "string"
	case protoreflect.MessageKind, protoreflect.GroupKind:
		fullName := string(field.Message().FullName())
		if t, ok := wellKnownTypes[fullName]; ok {
			return t
		}
		return "json.RawMessage"
	default:
		// All proto kinds are covered above; an unknown Kind means protoreflect
		// added a new one we haven't taught the plugin about yet. Panic loudly
		// rather than silently emit a misleading "string" type.
		panic(fmt.Sprintf("protoc-gen-mcp: unknown proto Kind %v on field %s", field.Kind(), field.FullName()))
	}
}
