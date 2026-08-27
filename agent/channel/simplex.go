package channel

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SimpleXChannel struct {
	cfg Config
}

type SimpleXEvent struct {
	Type      string            `json:"type,omitempty"`
	Resp      *SimpleXEventResp `json:"resp,omitempty"`
	ChatItems []SimpleXChatItem `json:"chatItems,omitempty"`
}

type SimpleXEventResp struct {
	Type      string            `json:"type,omitempty"`
	ChatItems []SimpleXChatItem `json:"chatItems,omitempty"`
}

type SimpleXChatItem struct {
	ID      string         `json:"id,omitempty"`
	Text    string         `json:"text,omitempty"`
	Content map[string]any `json:"content,omitempty"`
	Chat    SimpleXChat    `json:"chat,omitempty"`
}

type SimpleXChat struct {
	ID          string `json:"id,omitempty"`
	ContactID   string `json:"contactId,omitempty"`
	ContactName string `json:"contactName,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

func NewSimpleXChannel(cfg Config) SimpleXChannel {
	return SimpleXChannel{cfg: cfg}
}

func (c SimpleXChannel) Name() string {
	return c.cfg.Name
}

func (c SimpleXChannel) SendDraft(ctx context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	return c.send(ctx, draftBody("7review draft ready", msg))
}

func (c SimpleXChannel) SendFinalConfirmation(ctx context.Context, msg FinalConfirmationMessage) error {
	_, err := c.send(ctx, "7review final published\nrun: "+msg.RunID+"\n\n"+strings.TrimSpace(msg.FinalReport))
	return err
}

func (c SimpleXChannel) send(ctx context.Context, text string) (DeliveryReceipt, error) {
	contact := firstNonEmpty(Setting(c.cfg.Settings, "contact_id"), Setting(c.cfg.Settings, "contact_name"), firstSender(c.cfg.AuthorizedSenders))
	if contact == "" {
		return DeliveryReceipt{}, fmt.Errorf("simplex channel %s missing contact_id/contact_name", c.cfg.Name)
	}
	corrID := "7review-" + time.Now().UTC().Format("20060102150405.000000000")
	cmd := "/_send @" + contact + " " + strings.TrimSpace(text)
	if err := writeSimpleXCommand(ctx, simpleXURL(c.cfg), corrID, cmd); err != nil {
		return DeliveryReceipt{}, err
	}
	return DeliveryReceipt{Channel: c.cfg.Name, ExternalID: corrID}, nil
}

func (c SimpleXChannel) Start(ctx context.Context, handler func(InboundMessage)) error {
	backoff := time.Second
	for ctx.Err() == nil {
		err := readSimpleXEvents(ctx, simpleXURL(c.cfg), func(data []byte) {
			for _, msg := range ParseSimpleXInbound(c.cfg.Name, data) {
				if _, err := NewManager([]Config{c.cfg}).VerifyInbound(c.cfg.Name, "", msg); err != nil {
					log.Printf("[channel:%s] rejected simplex message: %v", c.cfg.Name, err)
					continue
				}
				handler(msg)
			}
		})
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("[channel:%s] simplex listener reconnecting after error: %v", c.cfg.Name, err)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
	return nil
}

func ParseSimpleXInbound(channelName string, data []byte) []InboundMessage {
	var event SimpleXEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil
	}
	eventType := firstNonEmpty(event.Type, eventRespType(event.Resp))
	if eventType != "NewChatItems" {
		return nil
	}
	items := event.ChatItems
	if len(items) == 0 && event.Resp != nil {
		items = event.Resp.ChatItems
	}
	var out []InboundMessage
	for _, item := range items {
		text := strings.TrimSpace(firstNonEmpty(item.Text, stringFromMap(item.Content, "text"), stringFromMap(item.Content, "msgContent", "text")))
		if text == "" {
			continue
		}
		senderID := firstNonEmpty(item.Chat.ContactID, item.Chat.ID)
		senderAddress := firstNonEmpty(item.Chat.ContactName, item.Chat.DisplayName)
		out = append(out, InboundMessage{
			Channel:       channelName,
			ExternalID:    strings.TrimSpace(item.ID),
			RunID:         RunIDFromCommand(text),
			SenderID:      senderID,
			SenderAddress: senderAddress,
			Text:          text,
			ReceivedAt:    time.Now().UTC(),
		})
	}
	return out
}

func eventRespType(resp *SimpleXEventResp) string {
	if resp == nil {
		return ""
	}
	return resp.Type
}

func stringFromMap(m map[string]any, path ...string) string {
	var current any = m
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = next[key]
	}
	value, _ := current.(string)
	return value
}

func simpleXURL(cfg Config) string {
	return firstNonEmpty(Setting(cfg.Settings, "ws_url"), "ws://127.0.0.1:5225")
}

func writeSimpleXCommand(ctx context.Context, rawURL string, corrID string, cmd string) error {
	conn, err := dialWebSocket(ctx, rawURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	payload, err := json.Marshal(map[string]string{"corrId": corrID, "cmd": cmd})
	if err != nil {
		return err
	}
	return writeWebSocketText(conn, payload)
}

func readSimpleXEvents(ctx context.Context, rawURL string, handler func([]byte)) error {
	conn, err := dialWebSocket(ctx, rawURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	for ctx.Err() == nil {
		data, err := readWebSocketText(conn)
		if err != nil {
			return err
		}
		handler(data)
	}
	return nil
}

func dialWebSocket(ctx context.Context, rawURL string) (net.Conn, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "ws" {
		return nil, fmt.Errorf("simplex only supports local ws URLs")
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+parsed.Host+path, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	req.Host = parsed.Host
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", key)
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade status %d", resp.StatusCode)
	}
	expectedAccept := websocketAccept(key)
	if resp.Header.Get("Sec-WebSocket-Accept") != expectedAccept {
		conn.Close()
		return nil, fmt.Errorf("invalid websocket accept")
	}
	return conn, nil
}

func websocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func writeWebSocketText(w io.Writer, payload []byte) error {
	header := []byte{0x81}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length)|0x80)
	case length <= 65535:
		header = append(header, 126|0x80, byte(length>>8), byte(length))
	default:
		header = append(header, 127|0x80)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		header = append(header, buf[:]...)
	}
	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)
	masked := make([]byte, length)
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(masked)
	return err
}

func readWebSocketText(r io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	opcode := header[0] & 0x0f
	if opcode == 0x8 {
		return nil, io.EOF
	}
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf[:]))
	case 127:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return nil, err
		}
		length = binary.BigEndian.Uint64(buf[:])
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	if opcode != 0x1 {
		return nil, fmt.Errorf("unexpected websocket opcode %d", opcode)
	}
	return payload, nil
}
