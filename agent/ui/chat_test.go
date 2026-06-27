package ui

import (
	"context"
	"fmt"
	"io"
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

func TestRunChat_PlainRunPromptShowsSession(t *testing.T) {
	responder := &fakeResponder{}
	var out strings.Builder
	err := RunChat(context.Background(), strings.NewReader("explain\nquit\n"), &out, ChatContext{
		ConfigLoaded: true,
		RunID:        "owner/repo!7",
		ServerURL:    "http://agent",
	}, responder, ChatOptions{Plain: true})
	if err != nil {
		t.Fatal(err)
	}
	output := out.String()
	for _, want := range []string{"run: owner/repo!7", "server: http://agent", "owner/repo!7> ", "agent: model reply: explain"} {
		if !strings.Contains(output, want) {
			t.Fatalf("run chat output missing %q:\n%s", want, output)
		}
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

func TestRunChat_HandlesLocalCommandWithoutCallingResponder(t *testing.T) {
	responder := &fakeResponder{}
	var out strings.Builder
	err := RunChat(context.Background(), strings.NewReader("/history\nquit\n"), &out, ChatContext{RunID: "owner/repo!7"}, responder, ChatOptions{
		Plain: true,
		CommandHandler: func(_ context.Context, text string, out io.Writer, _ ChatContext, opts ChatOptions) (bool, error) {
			if text != "/history" {
				t.Fatalf("unexpected command text %q", text)
			}
			fmt.Fprintln(out, RenderChatMessage(ChatMessage{Role: "agent", Text: "history output"}, opts.Plain))
			return true, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(responder.calls) != 0 {
		t.Fatalf("command should not call model responder: %#v", responder.calls)
	}
	if !strings.Contains(out.String(), "agent: history output") {
		t.Fatalf("command output missing:\n%s", out.String())
	}
}

func TestRenderChatIntroStyledUsesTerminalComposer(t *testing.T) {
	out := RenderChatIntro(ChatContext{
		ConfigLoaded: true,
		RunID:        "owner/repo!7",
		ServerURL:    "http://agent",
		HeadroomURL:  "http://headroom:8787",
		MemPalaceURL: "http://mempalace:8788",
	}, false)
	for _, want := range []string{"7review", "| ask about run owner/repo!7", "Chat  owner/repo!7", "tab switch agent", "Context", "mode      run", "run       owner/repo!7", "server    http://agent", "headroom  connected", "mempalace connected"} {
		if !strings.Contains(out, want) {
			t.Fatalf("styled chat intro missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "7review chat\nconfig loaded") {
		t.Fatalf("styled chat intro kept old banner layout:\n%s", out)
	}
}

func TestRenderChatMessagePrefixStyledUsesTerminalBlock(t *testing.T) {
	out := RenderChatMessagePrefix("agent", false)
	for _, want := range []string{"Build", "7review\n  "} {
		if !strings.Contains(out, want) {
			t.Fatalf("styled agent prefix missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "agent:") {
		t.Fatalf("styled prefix kept old REPL label:\n%s", out)
	}
}
