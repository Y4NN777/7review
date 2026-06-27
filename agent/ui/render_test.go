package ui

import (
	"strings"
	"testing"
)

func TestRenderStatusPlain(t *testing.T) {
	out := RenderStatus(StatusView{
		Title: "status",
		Plain: true,
		Dependencies: []DependencyStatus{
			{Name: "mempalace", URL: "http://m", Ready: false, Detail: "down"},
			{Name: "headroom", URL: "http://h", Ready: true},
		},
	})
	for _, want := range []string{"status", "headroom", "ok", "mempalace", "down"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in %q", want, out)
		}
	}
}

func TestRenderStatusStyled(t *testing.T) {
	out := RenderStatus(StatusView{
		Dependencies: []DependencyStatus{{Name: "headroom", URL: "http://h", Ready: true}},
	})
	if !strings.Contains(out, "7review status") || !strings.Contains(out, "headroom") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderStatusStyledDoesNotLeakANSICodeText(t *testing.T) {
	out := RenderStatus(StatusView{
		Dependencies: []DependencyStatus{
			{Name: "agent", URL: "http://agent/ready", Ready: true, Detail: "http=200"},
			{Name: "queue", Ready: true, Detail: "ok depth=0 capacity=32"},
		},
	})
	for _, leaked := range []string{"38;5;", "0m", "[38"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("status output leaked ANSI text %q:\n%q", leaked, out)
		}
	}
}
