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
	Plain          bool
	CommandHandler ChatCommandFunc
}

type ChatContext struct {
	ConfigLoaded bool
	ConfigError  string
	HeadroomURL  string
	MemPalaceURL string
	RunID        string
	ServerURL    string
}

type ChatCommandFunc func(context.Context, string, io.Writer, ChatContext, ChatOptions) (bool, error)

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
		fmt.Fprint(out, prompt(meta, opts.Plain))
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
		if handled, err := handleChatCommand(ctx, text, out, meta, opts); handled {
			if err != nil {
				fmt.Fprintln(out, RenderChatMessage(ChatMessage{Role: "agent", Text: "chat error: " + err.Error()}, opts.Plain))
			}
			continue
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

func handleChatCommand(ctx context.Context, text string, out io.Writer, meta ChatContext, opts ChatOptions) (bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(text), "/") || opts.CommandHandler == nil {
		return false, nil
	}
	return opts.CommandHandler(ctx, text, out, meta, opts)
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
	if ctx.RunID != "" {
		lines = append(lines, "run: "+ctx.RunID)
	}
	if ctx.ServerURL != "" {
		lines = append(lines, "server: "+ctx.ServerURL)
	}
	if ctx.ConfigError != "" {
		lines = append(lines, "config: "+ctx.ConfigError)
	}
	text := strings.Join(lines, "\n")
	if plain {
		return text
	}
	body := joinColumns(renderChatMain(status, ctx), renderChatRail(ctx), 2)
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#000000")).
		Foreground(lipgloss.Color("#D0D0D0")).
		Render(body)
}

func renderChatMain(status string, ctx ChatContext) string {
	lines := []string{
		"",
		centerText("7review", 72),
		"",
		"  " + status,
	}
	if ctx.ConfigError != "" {
		lines = append(lines, "  config: "+ctx.ConfigError)
	}
	lines = append(lines,
		"",
		"  "+composerLine(chatComposerText(ctx)),
		"  Chat  "+chatContextLabel(ctx),
		"",
		"                                            tab switch agent  ctrl+c commands",
	)
	return renderConsoleSurface(lines, 78, false)
}

func chatComposerText(ctx ChatContext) string {
	if ctx.RunID != "" {
		return "ask about run " + ctx.RunID
	}
	return "ask about setup, status, reviews, or next steps"
}

func chatContextLabel(ctx ChatContext) string {
	if ctx.RunID != "" {
		return trimTo(ctx.RunID, 32)
	}
	return "7review"
}

func renderChatRail(ctx ChatContext) string {
	mode := "local"
	if ctx.RunID != "" {
		mode = "run"
	}
	lines := []string{
		"Chat",
		"",
		"Context",
		"mode      " + mode,
	}
	if ctx.RunID != "" {
		lines = append(lines, "run       "+trimTo(ctx.RunID, 20))
	}
	if ctx.ServerURL != "" {
		lines = append(lines, "server    "+trimTo(ctx.ServerURL, 20))
	}
	if ctx.HeadroomURL != "" {
		lines = append(lines, "", "MCP", "headroom  connected")
	}
	if ctx.MemPalaceURL != "" {
		if ctx.HeadroomURL == "" {
			lines = append(lines, "", "MCP")
		}
		lines = append(lines, "mempalace connected")
	}
	if ctx.ConfigError != "" {
		lines = append(lines, "", "Config", trimTo(ctx.ConfigError, 28))
	}
	lines = append(lines, "", "~", "7review")
	return renderConsoleSurface(lines, 30, false)
}

func RenderChatMessage(msg ChatMessage, plain bool) string {
	return RenderChatMessagePrefix(msg.Role, plain) + msg.Text
}

func RenderChatMessagePrefix(role string, plain bool) string {
	if plain {
		return role + ": "
	}
	color := lipgloss.Color("#00E676")
	if role == "user" {
		color = lipgloss.Color("#4AA3FF")
	}
	label := "  " + role
	if role == "agent" {
		label = "  Build"
	}
	return lipgloss.NewStyle().Foreground(color).Render(label+"  ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("#5F6368")).Render("7review\n  ")
}

func prompt(ctx ChatContext, plain bool) string {
	if plain {
		if ctx.RunID != "" {
			return ctx.RunID + "> "
		}
		return "you> "
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("\n| ")
}

func composerLine(text string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D0D0D0")).
		Background(lipgloss.Color("#1A1A1A")).
		Render("| " + text)
}

func isQuit(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "q", "quit", "exit":
		return true
	default:
		return false
	}
}
