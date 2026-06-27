package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type ChatOptions struct {
	Plain bool
}

type ChatContext struct {
	ConfigLoaded bool
	ConfigError  string
	HeadroomURL  string
	MemPalaceURL string
}

type ChatResponder interface {
	StreamRespond(context.Context, string, func(string) error) error
}

type StaticResponder struct {
	Message string
}

func (r StaticResponder) StreamRespond(_ context.Context, _ string, emit func(string) error) error {
	return emit(r.Message)
}

type ChatMessage struct {
	Role string
	Text string
}

func RunChat(ctx context.Context, in io.Reader, out io.Writer, meta ChatContext, responder ChatResponder, opts ChatOptions) error {
	if responder == nil {
		return fmt.Errorf("chat responder is required")
	}
	fmt.Fprintln(out, RenderChatIntro(meta, opts.Plain))
	reader := bufio.NewReader(in)
	for {
		fmt.Fprint(out, prompt(opts.Plain))
		line, err := reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			if err == io.EOF {
				return nil
			}
			return err
		}
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if isQuit(text) {
			fmt.Fprintln(out, RenderChatMessage(ChatMessage{Role: "agent", Text: "Bye."}, opts.Plain))
			return nil
		}
		fmt.Fprint(out, RenderChatMessagePrefix("agent", opts.Plain))
		err = responder.StreamRespond(ctx, text, func(delta string) error {
			_, writeErr := fmt.Fprint(out, delta)
			return writeErr
		})
		fmt.Fprintln(out)
		if err != nil {
			fmt.Fprintln(out, RenderChatMessage(ChatMessage{Role: "agent", Text: "chat error: " + err.Error()}, opts.Plain))
			continue
		}
	}
}

func RenderChatIntro(ctx ChatContext, plain bool) string {
	status := "config loaded"
	if !ctx.ConfigLoaded {
		status = "config needs attention"
	}
	lines := []string{
		"7review chat",
		status,
		"Ask about setup, status, Docker, sidecars, webhooks, or next steps. Type quit to exit.",
	}
	if ctx.ConfigError != "" {
		lines = append(lines, "config: "+ctx.ConfigError)
	}
	text := strings.Join(lines, "\n")
	if plain {
		return text
	}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(lines[0])
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(strings.Join(lines[1:], "\n"))
	return title + "\n" + body
}

func RenderChatMessage(msg ChatMessage, plain bool) string {
	return RenderChatMessagePrefix(msg.Role, plain) + msg.Text
}

func RenderChatMessagePrefix(role string, plain bool) string {
	if plain {
		return role + ": "
	}
	color := lipgloss.Color("42")
	if role == "user" {
		color = lipgloss.Color("39")
	}
	return lipgloss.NewStyle().Foreground(color).Render(role + ": ")
}

func prompt(plain bool) string {
	if plain {
		return "you> "
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("you> ")
}

func isQuit(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "q", "quit", "exit":
		return true
	default:
		return false
	}
}
