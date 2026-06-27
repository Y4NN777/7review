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
