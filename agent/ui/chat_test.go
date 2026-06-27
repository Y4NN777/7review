package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type fakeResponder struct {
	calls []string
	err   error
}

func (r *fakeResponder) StreamRespond(_ context.Context, input string, emit func(string) error) error {
	r.calls = append(r.calls, input)
	if r.err != nil {
		return r.err
	}
	if err := emit("model "); err != nil {
		return err
	}
	return emit("reply: " + input)
}

func TestRunChat_UsesResponderAndExitsOnQuit(t *testing.T) {
	responder := &fakeResponder{}
	var out strings.Builder
	err := RunChat(context.Background(), strings.NewReader("help\nquit\n"), &out, ChatContext{}, responder, ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	if !strings.Contains(output, "7review chat") || !strings.Contains(output, "agent: model reply: help") || !strings.Contains(output, "Bye.") {
		t.Fatalf("unexpected chat output:\n%s", output)
	}
	if len(responder.calls) != 1 || responder.calls[0] != "help" {
		t.Fatalf("unexpected responder calls: %#v", responder.calls)
	}
}

func TestRunChat_RendersResponderError(t *testing.T) {
	responder := &fakeResponder{err: fmt.Errorf("model unavailable")}
	var out strings.Builder
	err := RunChat(context.Background(), strings.NewReader("hello\nquit\n"), &out, ChatContext{}, responder, ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "chat error: model unavailable") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestRenderChatIntroStyledUsesTerminalComposer(t *testing.T) {
	out := RenderChatIntro(ChatContext{ConfigLoaded: true}, false)
	for _, want := range []string{"7review", "| ask about setup, status, reviews, or next steps", "Chat  7review", "tab switch agent"} {
		if !strings.Contains(out, want) {
			t.Fatalf("styled chat intro missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "7review chat\nconfig loaded") {
		t.Fatalf("styled chat intro kept old banner layout:\n%s", out)
	}
}
