package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// withURL redirects a vendor endpoint var to a test server.
func withURL(t *testing.T, target *string, url string) {
	t.Helper()
	old := *target
	*target = url
	t.Cleanup(func() { *target = old })
}

const webFixture = `<!doctype html>
<html><head><title>Ignored Title</title>
<style>.x{color:red}</style>
<script>if (a < b) { location.href = "/x"; } // > comment trap</script>
</head>
<body>
<!-- a > b comment -->
<h1>Hello &amp; welcome</h1>
<p>Intro <b>bold</b> and <i>ital</i>.</p>
<p>See <a href="https://example.com/docs">the docs</a> for more.</p>
<ul><li>one</li><li>two</li></ul>
<pre><code>go test ./...</code></pre>
<table><tr><th>A</th><td>1</td></tr></table>
<img src="/pic.png" alt="pic">
<p>Break<br>after.</p>
</body></html>`

func TestWebFetchHTMLToMarkdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, webFixture)
	}))
	defer srv.Close()
	r := webTool{}.Run(context.Background(), &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/page"}`))
	if r.IsError {
		t.Fatalf("fetch failed: %s", r.Content)
	}
	for _, want := range []string{
		"# Ignored Title",
		"# Hello & welcome",
		"Intro **bold** and *ital*.",
		"[the docs](https://example.com/docs)",
		"- one\n- two",
		"```\ngo test ./...\n```",
		"A | 1 |",
		"![pic](/pic.png)",
		"Break\nafter.",
	} {
		if !strings.Contains(r.Content, want) {
			t.Errorf("markdown missing %q in:\n%s", want, r.Content)
		}
	}
	for _, bad := range []string{"location.href", ".x{color", "b comment", "<h1>"} {
		if strings.Contains(r.Content, bad) {
			t.Errorf("markdown kept %q in:\n%s", bad, r.Content)
		}
	}
	if !strings.Contains(r.Summary, "markdown") || !strings.Contains(r.Summary, "page") {
		t.Errorf("summary: %q", r.Summary)
	}
}

func TestWebFetchFormats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true}`)
		case "/text":
			w.Header().Set("Content-Type", "text/plain")
			io.WriteString(w, "plain body")
		default:
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, webFixture)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	r := webTool{}.Run(ctx, &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/page","format":"text"}`))
	if r.IsError || !strings.Contains(r.Content, "Intro bold and ital.") || strings.Contains(r.Content, "**") {
		t.Errorf("text format: %+v", r)
	}
	r = webTool{}.Run(ctx, &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/page","format":"html"}`))
	if r.IsError || !strings.Contains(r.Content, "<h1>Hello") {
		t.Errorf("html format: %+v", r)
	}
	r = webTool{}.Run(ctx, &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/json"}`))
	if r.IsError || r.Content != `{"ok":true}` {
		t.Errorf("json passthrough: %+v", r)
	}
	r = webTool{}.Run(ctx, &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/text"}`))
	if r.IsError || r.Content != "plain body" {
		t.Errorf("text passthrough: %+v", r)
	}
}

func TestWebFetchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/missing":
			http.Error(w, "gone fishing", http.StatusNotFound)
		case "/png":
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "\x89PNG")
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/big":
			w.Header().Set("Content-Type", "text/html")
			io.WriteString(w, "<p>"+strings.Repeat("word ", 20000)+"</p>")
		default:
			io.WriteString(w, "ok")
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	cases := []struct{ in, want string }{
		{`{"op":"fetch","url":"` + srv.URL + `/missing"}`, "http 404"},
		{`{"op":"fetch","url":"` + srv.URL + `/png"}`, "unsupported content type"},
		{`{"op":"fetch","url":"` + srv.URL + `/loop"}`, "redirects"},
		{`{"op":"fetch","url":"ftp://x/y"}`, "absolute http(s)"},
		{`{"op":"fetch","url":"notaurl"}`, "absolute http(s)"},
		{`{"op":"fetch","url":"` + srv.URL + `","format":"pdf"}`, "format must be"},
		{`{"op":"fetch","url":"` + srv.URL + `","timeout":0}`, "timeout must be"},
		{`{"op":"fetch","url":"` + srv.URL + `","timeout":999}`, "timeout must be"},
		{`{"op":"fetch"}`, "absolute http(s)"},
		{`{"op":"nope"}`, `op must be "fetch" or "search"`},
	}
	for _, c := range cases {
		r := webTool{}.Run(ctx, &Env{}, json.RawMessage(c.in))
		if !r.IsError || !strings.Contains(r.Content, c.want) {
			t.Errorf("%s: want error containing %q, got %+v", c.in, c.want, r)
		}
	}
	r := webTool{}.Run(ctx, &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/big"}`))
	if r.IsError || !strings.Contains(r.Content, "truncated:") {
		t.Errorf("big page should be truncated, got err=%v len=%d", r.IsError, len(r.Content))
	}
}

func TestWebFetchTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)
	r := webTool{}.Run(context.Background(), &Env{}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`","timeout":1}`))
	if !r.IsError || !strings.Contains(r.Content, "fetch failed") {
		t.Fatalf("want timeout error, got %+v", r)
	}
}

func TestWebSearchTavilyKeyless(t *testing.T) {
	var gotKeyless, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKeyless = r.Header.Get("X-Tavily-Access-Mode")
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"results":[{"title":"Result One","url":"https://a.example","content":"first snippet"},{"title":"Two","url":"https://b.example","content":"second"}]}`)
	}))
	defer srv.Close()
	withURL(t, &webTavilyURL, srv.URL)
	r := webTool{}.Run(context.Background(), &Env{}, json.RawMessage(`{"op":"search","query":"golang testing"}`))
	if r.IsError {
		t.Fatalf("search failed: %s", r.Content)
	}
	if gotKeyless != "keyless" || gotAuth != "" {
		t.Errorf("keyless mode headers: keyless=%q auth=%q", gotKeyless, gotAuth)
	}
	if !strings.Contains(gotBody, `"query":"golang testing"`) || !strings.Contains(gotBody, `"search_depth":"basic"`) || !strings.Contains(gotBody, `"max_results":5`) {
		t.Errorf("request body: %s", gotBody)
	}
	want := "1. Result One\n   https://a.example\n   first snippet\n2. Two\n   https://b.example\n   second"
	if r.Content != want {
		t.Errorf("content:\n%s\nwant:\n%s", r.Content, want)
	}
	if r.Summary != `"golang testing" (2 results, tavily keyless)` {
		t.Errorf("summary: %q", r.Summary)
	}

	r = webTool{}.Run(context.Background(), &Env{WebKey: "tvly-secret"}, json.RawMessage(`{"op":"search","query":"q","maxResults":2}`))
	if r.IsError {
		t.Fatalf("keyed search failed: %s", r.Content)
	}
	if gotKeyless != "" || gotAuth != "Bearer tvly-secret" {
		t.Errorf("keyed mode headers: keyless=%q auth=%q", gotKeyless, gotAuth)
	}
	if !strings.Contains(gotBody, `"max_results":2`) {
		t.Errorf("maxResults not forwarded: %s", gotBody)
	}
	if !strings.Contains(r.Summary, "tavily key") {
		t.Errorf("summary should say key mode: %q", r.Summary)
	}
}

func TestWebSearchExa(t *testing.T) {
	var gotKey, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"results":[{"title":"E1","url":"https://e.example","text":"exa snippet"}]}`)
	}))
	defer srv.Close()
	withURL(t, &webExaURL, srv.URL)
	env := &Env{WebProvider: "exa", WebKey: "exa-key"}
	r := webTool{}.Run(context.Background(), env, json.RawMessage(`{"op":"search","query":"q","maxResults":3}`))
	if r.IsError {
		t.Fatalf("exa search failed: %s", r.Content)
	}
	if gotKey != "exa-key" || !strings.Contains(gotBody, `"numResults":3`) || !strings.Contains(gotBody, `"maxCharacters":300`) {
		t.Errorf("request: key=%q body=%s", gotKey, gotBody)
	}
	if !strings.Contains(r.Content, "E1") || !strings.Contains(r.Content, "exa snippet") {
		t.Errorf("content: %s", r.Content)
	}
	// Without a key exa refuses, naming the config key.
	r = webTool{}.Run(context.Background(), &Env{WebProvider: "exa"}, json.RawMessage(`{"op":"search","query":"q"}`))
	if !r.IsError || !strings.Contains(r.Content, "web.search.apiKey") {
		t.Errorf("keyless exa should refuse: %+v", r)
	}
}

func TestWebSearchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"detail":{"error":"invalid api key"}}`)
	}))
	defer srv.Close()
	withURL(t, &webTavilyURL, srv.URL)
	ctx := context.Background()
	for _, c := range []struct{ in, want string }{
		{`{"op":"search"}`, "query is required"},
		{`{"op":"search","query":" ","maxResults":0}`, "query is required"},
		{`{"op":"search","query":"q","maxResults":11}`, "maxResults must be"},
		{`{"op":"search","query":"q","maxResults":0}`, "maxResults must be"},
		{`{"op":"search","query":"q"}`, "http 401"},
	} {
		r := webTool{}.Run(ctx, &Env{}, json.RawMessage(c.in))
		if !r.IsError || !strings.Contains(r.Content, c.want) {
			t.Errorf("%s: want %q, got %+v", c.in, c.want, r)
		}
	}
	r := webTool{}.Run(ctx, &Env{WebProvider: "bing"}, json.RawMessage(`{"op":"search","query":"q"}`))
	if !r.IsError || !strings.Contains(r.Content, "unknown") {
		t.Errorf("unknown provider: %+v", r)
	}
}

func TestWebSearchEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"results":[]}`)
	}))
	defer srv.Close()
	withURL(t, &webTavilyURL, srv.URL)
	r := webTool{}.Run(context.Background(), &Env{}, json.RawMessage(`{"op":"search","query":"zzz"}`))
	if r.IsError || r.Content != "no results" || !strings.Contains(r.Summary, "0 results") {
		t.Errorf("empty results: %+v", r)
	}
}

func TestWebHTMLConverters(t *testing.T) {
	// Inline boundaries stay separate words.
	if got := webHTMLToMarkdown(`<p>Hello <b>world</b>!</p>`); !strings.Contains(got, "Hello **world**!") {
		t.Errorf("inline boundary: %q", got)
	}
	// Unknown elements degrade to text; comments and script with a `<` survive nothing.
	got := webHTMLToMarkdown(`<div><custom-tag>x</custom-tag><!-- drop <me> --><script>a<b</script>y</div>`)
	if !strings.Contains(got, "x") || !strings.Contains(got, "y") || strings.Contains(got, "drop") || strings.Contains(got, "a<b") {
		t.Errorf("degrade: %q", got)
	}
	// NBSP becomes a plain space.
	if got := webHTMLToText("<p>a&nbsp;b</p>"); got != "a b" {
		t.Errorf("nbsp: %q", got)
	}
	// Rune-safe truncation never splits a multi-byte rune.
	s := strings.Repeat("é", 10)
	if cut := cutWeb(s, 5); cut != "éé" {
		t.Errorf("cutWeb runes: %q", cut)
	}
}

// TestWebFetchWireCapIsReported: a body past the 2 MiB read cap is cut, and
// the result says so even when the rendered page fits the output limit —
// otherwise the model takes a partial page for the whole one.
func TestWebFetchWireCapIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "<p>start</p><script>"+strings.Repeat("x", webReadLimit)+"</script><p>end</p>")
	}))
	defer srv.Close()
	env, _ := testEnv(t)
	r := webTool{}.Run(context.Background(), env, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`"}`))
	if r.IsError || !strings.Contains(r.Content, "start") || !strings.Contains(r.Content, "read cap") {
		t.Fatalf("wire cap not reported: %.200q", r.Content)
	}
}

// TestWebFetchOverflowSpills: rendered output past 20K chars is cut for the
// model and saved in full.
func TestWebFetchOverflowSpills(t *testing.T) {
	body := strings.Repeat("word ", 10_000) + "THE-END"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, body)
	}))
	defer srv.Close()
	env, _ := testEnv(t)
	env.SpillDir = t.TempDir()
	r := webTool{}.Run(context.Background(), env, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`"}`))
	if r.IsError || len(r.Content) > webOutputMax+600 || strings.Contains(r.Content, "THE-END") {
		t.Fatalf("cut: %d chars", len(r.Content))
	}
	b, err := os.ReadFile(spilledPath(t, r.Content))
	if err != nil || !strings.HasSuffix(string(b), "THE-END") {
		t.Fatalf("spilled: %v", err)
	}
}

// TestWebHTMLStrayClosingPre: an unmatched </pre> must not leave the
// renderer unable to enter preformatted mode for a later block.
func TestWebHTMLStrayClosingPre(t *testing.T) {
	got := webHTMLToText("<p>intro</pre></p><pre>a  b\n  c</pre>")
	if !strings.Contains(got, "a  b\n  c") {
		t.Fatalf("later <pre> lost its whitespace: %q", got)
	}
}
