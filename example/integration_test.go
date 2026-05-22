// Package example_test contains end-to-end integration tests for the generated
// gRPC → MCP bridge code.
//
// Five layers are exercised:
//
//  1. gRPC server (direct): verifies the example Greeter implementation.
//  2. MCP tools over in-memory transport: verifies the generated
//     RegisterGreeterServiceTools wiring without any network overhead.
//  3. MCP tools over HTTP transport: verifies the full generated mcp-server
//     stack using mcp.NewStreamableHTTPHandler and mcp.StreamableClientTransport.
//  4. Aggregator Run over HTTP: verifies the standalone-binary path
//     (mcpserver.Run dials remote gRPC, serves MCP-HTTP).
//  5. Aggregator RegisterLocal: verifies the embedded path — MCP tools
//     dispatch straight into a server impl with no gRPC client.
package example_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "github.com/andreas-04/protoc-gen-mcp/example/gen/greeter"
	"github.com/andreas-04/protoc-gen-mcp/example/gen/mcpserver"
	"github.com/andreas-04/protoc-gen-mcp/example/internal/greeterimpl"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// startGRPCServer starts an in-process gRPC server on a random port and
// returns its address plus a stop function.
func startGRPCServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterGreeterServiceServer(srv, greeterimpl.Server{})
	go srv.Serve(ln) //nolint:errcheck
	return ln.Addr().String(), srv.GracefulStop
}

// grpcClient dials the given address and returns a GreeterServiceClient plus
// a close function.
func grpcClient(t *testing.T, addr string) (pb.GreeterServiceClient, func()) {
	t.Helper()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	return pb.NewGreeterServiceClient(conn), func() { conn.Close() }
}

// ── Layer 1: gRPC direct ──────────────────────────────────────────────────────

func TestGRPC_SayHello_Default(t *testing.T) {
	addr, stop := startGRPCServer(t)
	defer stop()
	client, close := grpcClient(t, addr)
	defer close()

	resp, err := client.SayHello(context.Background(), &pb.SayHelloRequest{Name: "World"})
	if err != nil {
		t.Fatalf("SayHello: %v", err)
	}
	want := "Hello, World!"
	if resp.Message != want {
		t.Errorf("got %q; want %q", resp.Message, want)
	}
}

func TestGRPC_SayHello_Languages(t *testing.T) {
	addr, stop := startGRPCServer(t)
	defer stop()
	client, close := grpcClient(t, addr)
	defer close()

	cases := []struct {
		lang, want string
	}{
		{"es", "Hola, Ana!"},
		{"fr", "Bonjour, Ana!"},
		{"de", "Hallo, Ana!"},
		{"jp", "Hello, Ana!"}, // unknown → fallback to English
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			resp, err := client.SayHello(context.Background(), &pb.SayHelloRequest{Name: "Ana", Language: tc.lang})
			if err != nil {
				t.Fatalf("SayHello(%q): %v", tc.lang, err)
			}
			if resp.Message != tc.want {
				t.Errorf("got %q; want %q", resp.Message, tc.want)
			}
		})
	}
}

func TestGRPC_SayGoodbye(t *testing.T) {
	addr, stop := startGRPCServer(t)
	defer stop()
	client, close := grpcClient(t, addr)
	defer close()

	resp, err := client.SayGoodbye(context.Background(), &pb.SayGoodbyeRequest{Name: "World"})
	if err != nil {
		t.Fatalf("SayGoodbye: %v", err)
	}
	want := "Goodbye, World!"
	if resp.Message != want {
		t.Errorf("got %q; want %q", resp.Message, want)
	}
}

// ── Layer 2: MCP over in-memory transport ─────────────────────────────────────

// connectMCPInMemory creates a gRPC+MCP stack entirely in-process using
// mcp.NewInMemoryTransports, mirroring the RegisterGreeterServiceTools wiring.
func connectMCPInMemory(t *testing.T) (session *mcp.ClientSession, cleanup func()) {
	t.Helper()

	grpcAddr, stopGRPC := startGRPCServer(t)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "greeter-mcp", Version: "1.0.0"}, nil)
	pb.RegisterGreeterServiceTools(mcpSrv, pb.NewGreeterServiceClient(conn))

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("mcp server Connect: %v", err)
	}

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	sess, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("mcp client Connect: %v", err)
	}

	return sess, func() {
		sess.Close()
		conn.Close()
		stopGRPC()
	}
}

func TestMCPInMemory_ListTools(t *testing.T) {
	sess, cleanup := connectMCPInMemory(t)
	defer cleanup()

	result, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	wantNames := map[string]string{
		"greeter_service_say_hello":   "SayHello sends a personalised greeting to the named person.",
		"greeter_service_say_goodbye": "SayGoodbye sends a farewell message to the named person.",
	}

	if len(result.Tools) != len(wantNames) {
		t.Fatalf("got %d tools; want %d", len(result.Tools), len(wantNames))
	}
	for _, tool := range result.Tools {
		wantDesc, ok := wantNames[tool.Name]
		if !ok {
			t.Errorf("unexpected tool %q", tool.Name)
			continue
		}
		if tool.Description != wantDesc {
			t.Errorf("tool %q description: got %q; want %q", tool.Name, tool.Description, wantDesc)
		}
	}
}

func TestMCPInMemory_SayHello(t *testing.T) {
	sess, cleanup := connectMCPInMemory(t)
	defer cleanup()

	cases := []struct {
		name, lang, wantPrefix string
	}{
		{"World", "", "Hello, World!"},
		{"Mundo", "es", "Hola, Mundo!"},
		{"Monde", "fr", "Bonjour, Monde!"},
		{"Welt", "de", "Hallo, Welt!"},
	}
	for _, tc := range cases {
		t.Run(tc.lang+"/"+tc.name, func(t *testing.T) {
			res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "greeter_service_say_hello",
				Arguments: map[string]any{"name": tc.name, "language": tc.lang},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("tool returned error: %v", res.Content)
			}
			text := toolText(t, res)
			if !strings.Contains(text, tc.wantPrefix) {
				t.Errorf("response %q does not contain %q", text, tc.wantPrefix)
			}
		})
	}
}

// TestMCPInMemory_SayHello_OptionalTitle verifies that the proto3 'optional'
// keyword survives the MCP→gRPC round trip: when the MCP client omits the
// 'title' field, the gRPC server sees req.Title == nil. When the client
// supplies it, the value reaches the impl.
func TestMCPInMemory_SayHello_OptionalTitle(t *testing.T) {
	sess, cleanup := connectMCPInMemory(t)
	defer cleanup()

	// Title omitted: greeter should NOT prefix.
	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_hello",
		Arguments: map[string]any{"name": "Pat", "language": ""},
	})
	if err != nil {
		t.Fatalf("CallTool (no title): %v", err)
	}
	if got := toolText(t, res); !strings.Contains(got, "Hello, Pat!") || strings.Contains(got, "Dr.") {
		t.Errorf("unexpected response without title: %s", got)
	}

	// Title present: greeter prefixes.
	res, err = sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_hello",
		Arguments: map[string]any{"name": "Pat", "language": "", "title": "Dr."},
	})
	if err != nil {
		t.Fatalf("CallTool (with title): %v", err)
	}
	if got := toolText(t, res); !strings.Contains(got, "Hello, Dr. Pat!") {
		t.Errorf("unexpected response with title: %s", got)
	}
}

func TestMCPInMemory_SayGoodbye(t *testing.T) {
	sess, cleanup := connectMCPInMemory(t)
	defer cleanup()

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_goodbye",
		Arguments: map[string]any{"name": "Bob"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}
	text := toolText(t, res)
	if !strings.Contains(text, "Goodbye") || !strings.Contains(text, "Bob") {
		t.Errorf("unexpected response: %s", text)
	}
}

// ── Layer 3: MCP over HTTP (StreamableHTTPHandler) ────────────────────────────

// connectMCPHTTP starts the generated HTTP handler via httptest and connects
// a StreamableClientTransport, replicating the cmd/mcp-server setup exactly.
func connectMCPHTTP(t *testing.T) (session *mcp.ClientSession, cleanup func()) {
	t.Helper()

	grpcAddr, stopGRPC := startGRPCServer(t)

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	// Mirrors the generated mcp-server main exactly.
	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		s := mcp.NewServer(&mcp.Implementation{Name: "greeter-mcp", Version: "1.0.0"}, nil)
		pb.RegisterGreeterServiceTools(s, pb.NewGreeterServiceClient(conn))
		return s
	}, nil)

	httpSrv := httptest.NewServer(handler)

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             httpSrv.URL,
		DisableStandaloneSSE: true, // keep test connections simple
	}

	ctx := context.Background()
	sess, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("mcp client Connect (HTTP): %v", err)
	}

	return sess, func() {
		sess.Close()
		httpSrv.Close()
		conn.Close()
		stopGRPC()
	}
}

func TestMCPHTTP_ListTools(t *testing.T) {
	sess, cleanup := connectMCPHTTP(t)
	defer cleanup()

	result, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("got %d tools; want 2", len(result.Tools))
	}
}

func TestMCPHTTP_SayHello(t *testing.T) {
	sess, cleanup := connectMCPHTTP(t)
	defer cleanup()

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_hello",
		Arguments: map[string]any{"name": "Alice", "language": "fr"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}
	text := toolText(t, res)
	if !strings.Contains(text, "Bonjour") || !strings.Contains(text, "Alice") {
		t.Errorf("unexpected response: %s", text)
	}
}

func TestMCPHTTP_SayGoodbye(t *testing.T) {
	sess, cleanup := connectMCPHTTP(t)
	defer cleanup()

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_goodbye",
		Arguments: map[string]any{"name": "Charlie"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}
	text := toolText(t, res)
	if !strings.Contains(text, "Goodbye") || !strings.Contains(text, "Charlie") {
		t.Errorf("unexpected response: %s", text)
	}
}

// ── Layer 4: aggregator mcpserver.Run + Register ─────────────────────────────

// TestAggregator_Register verifies the generated mcpserver.Register helper
// wires every tool against an arbitrary *mcp.Server. This is the path users
// take when they manage their own MCP server lifecycle.
func TestAggregator_Register(t *testing.T) {
	grpcAddr, stopGRPC := startGRPCServer(t)
	defer stopGRPC()

	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	srv := mcp.NewServer(&mcp.Implementation{Name: "agg-test", Version: "0.0.0"}, nil)
	mcpserver.Register(srv, conn)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("mcp Connect: %v", err)
	}
	cli := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	sess, err := cli.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("mcp client Connect: %v", err)
	}
	defer sess.Close()

	got, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(got.Tools) != 2 {
		t.Fatalf("got %d tools; want 2", len(got.Tools))
	}
}

// TestAggregator_RunHTTP boots the standalone mcpserver.Run pipeline on a
// random HTTP address and connects an MCP client over it, end-to-end. This
// is the path users get when their main() calls mcpserver.Main().
func TestAggregator_RunHTTP(t *testing.T) {
	grpcAddr, stopGRPC := startGRPCServer(t)
	defer stopGRPC()

	httpAddr := pickFreeAddr(t)

	cfg := mcpserver.DefaultConfig()
	cfg.GRPCAddr = grpcAddr
	cfg.HTTPAddr = httpAddr

	runCtx, runCancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- mcpserver.Run(runCtx, cfg) }()

	sess := connectMCPWithRetry(t, "http://127.0.0.1"+httpAddr)
	defer sess.Close()

	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "greeter_service_say_hello",
		Arguments: map[string]any{"name": "Run", "language": "de"},
	})
	if err != nil {
		runCancel()
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		runCancel()
		t.Fatalf("tool returned error: %v", res.Content)
	}
	if got := toolText(t, res); !strings.Contains(got, "Hallo, Run!") {
		t.Errorf("unexpected response: %s", got)
	}

	runCancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("mcpserver.Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("mcpserver.Run did not shut down within 2s of context cancel")
	}
}

// ── Layer 5: aggregator RegisterLocal (embedded pattern) ──────────────────────

// TestAggregator_RegisterLocal verifies the embedded pattern: an MCP client
// calls a tool, and the tool dispatches directly into the server impl with
// no gRPC server, no gRPC client, no network.
func TestAggregator_RegisterLocal(t *testing.T) {
	mcpSrv := mcpserver.NewServer()
	mcpserver.RegisterLocal(mcpSrv, mcpserver.Impls{GreeterService: greeterimpl.Server{}})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("mcp Connect: %v", err)
	}
	cli := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	sess, err := cli.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("mcp client Connect: %v", err)
	}
	defer sess.Close()

	res, err := sess.CallTool(ctx, &mcp.CallToolParams{
		Name:      "greeter_service_say_hello",
		Arguments: map[string]any{"name": "Local", "language": "fr"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}
	if got := toolText(t, res); !strings.Contains(got, "Bonjour, Local!") {
		t.Errorf("unexpected response: %s", got)
	}
}

// TestAggregator_RegisterLocal_NilFieldSkipped confirms that a zero-valued
// Impls field doesn't register tools for that service. Lets callers wire a
// subset of services without nil dereferences inside the tool handlers.
func TestAggregator_RegisterLocal_NilFieldSkipped(t *testing.T) {
	mcpSrv := mcpserver.NewServer()
	mcpserver.RegisterLocal(mcpSrv, mcpserver.Impls{}) // no impls

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("mcp Connect: %v", err)
	}
	cli := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	sess, err := cli.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("mcp client Connect: %v", err)
	}
	defer sess.Close()

	got, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(got.Tools) != 0 {
		t.Fatalf("got %d tools; want 0 (all impls nil)", len(got.Tools))
	}
}

// ── utility ───────────────────────────────────────────────────────────────────

// pickFreeAddr returns ":N" for a random free TCP port.
func pickFreeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return fmt.Sprintf(":%d", port)
}

// connectMCPWithRetry connects an MCP client to endpoint, retrying for up
// to two seconds — the goroutine that owns the listener may not have bound
// it yet when we first try.
func connectMCPWithRetry(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	cli := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: endpoint, DisableStandaloneSSE: true}
	deadline := time.Now().Add(2 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		sess, err := cli.Connect(ctx, transport, nil)
		cancel()
		if err == nil {
			return sess
		}
		if time.Now().After(deadline) {
			t.Fatalf("mcp client Connect %s: %v", endpoint, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ── utility ───────────────────────────────────────────────────────────────────

// toolText extracts the text from the first TextContent in a CallToolResult.
func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("CallToolResult has no content")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}
