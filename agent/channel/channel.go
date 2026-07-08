package channel

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Name              string
	Provider          string
	Enabled           bool
	InboundToken      string
	AuthorizedSenders []string
	Settings          map[string]string
}

type DraftMessage struct {
	RunID       string
	Provider    string
	Repository  string
	ChangeID    string
	WebURL      string
	Summary     string
	DraftReport string
}

type FinalConfirmationMessage struct {
	RunID       string
	FinalReport string
}

type DeliveryReceipt struct {
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id,omitempty"`
	URL        string `json:"url,omitempty"`
}

type InboundMessage struct {
	Channel       string    `json:"channel,omitempty"`
	ExternalID    string    `json:"external_id,omitempty"`
	RunID         string    `json:"run_id"`
	SenderID      string    `json:"sender_id,omitempty"`
	SenderAddress string    `json:"sender_address,omitempty"`
	Text          string    `json:"text"`
	ReceivedAt    time.Time `json:"received_at,omitempty"`
}

type InboundResult struct {
	RunID       string `json:"run_id"`
	Channel     string `json:"channel"`
	Sender      string `json:"sender"`
	Command     string `json:"command"`
	Approved    bool   `json:"approved"`
	Revised     bool   `json:"revised,omitempty"`
	Suppressed  bool   `json:"suppressed,omitempty"`
	FindingID   string `json:"finding_id,omitempty"`
	Reason      string `json:"reason,omitempty"`
	FinalReport string `json:"final_report,omitempty"`
	Request     string `json:"request,omitempty"`
}

type NotificationChannel interface {
	Name() string
	SendDraft(context.Context, DraftMessage) (DeliveryReceipt, error)
	SendFinalConfirmation(context.Context, FinalConfirmationMessage) error
}

type LogChannel struct {
	name string
}

func NewLogChannel(name string) LogChannel {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "log"
	}
	return LogChannel{name: name}
}

func (c LogChannel) Name() string {
	return c.name
}

func (c LogChannel) SendDraft(_ context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	log.Printf("[channel:%s] draft ready run=%s provider=%s change=%s url=%s bytes=%d", c.name, msg.RunID, msg.Provider, msg.ChangeID, msg.WebURL, len(msg.DraftReport))
	return DeliveryReceipt{Channel: c.name, ExternalID: "log:" + msg.RunID}, nil
}

func (c LogChannel) SendFinalConfirmation(_ context.Context, msg FinalConfirmationMessage) error {
	log.Printf("[channel:%s] final published run=%s bytes=%d", c.name, msg.RunID, len(msg.FinalReport))
	return nil
}

type Manager struct {
	configs  map[string]Config
	channels map[string]NotificationChannel
}

func NewManager(configs []Config) *Manager {
	m := &Manager{
		configs:  make(map[string]Config),
		channels: make(map[string]NotificationChannel),
	}
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		name := normalizeName(firstNonEmpty(cfg.Name, cfg.Provider))
		if name == "" {
			continue
		}
		cfg.Name = name
		cfg.Provider = normalizeName(firstNonEmpty(cfg.Provider, "log"))
		cfg.AuthorizedSenders = cleanSenders(cfg.AuthorizedSenders)
		m.configs[name] = cfg
		switch cfg.Provider {
		case "twilio_whatsapp":
			m.channels[name] = NewTwilioWhatsAppChannel(cfg)
		case "sendgrid_email":
			m.channels[name] = NewSendGridEmailChannel(cfg)
		case "mailgun_email":
			m.channels[name] = NewMailgunEmailChannel(cfg)
		case "log":
			m.channels[name] = NewLogChannel(name)
		default:
			m.channels[name] = NewLogChannel(name)
		}
	}
	return m
}

func (m *Manager) Enabled() bool {
	return m != nil && len(m.channels) > 0
}

func (m *Manager) SendDraft(ctx context.Context, msg DraftMessage) ([]DeliveryReceipt, error) {
	if m == nil {
		return nil, nil
	}
	var receipts []DeliveryReceipt
	for name, ch := range m.channels {
		receipt, err := ch.SendDraft(ctx, msg)
		if err != nil {
			return receipts, fmt.Errorf("channel %s send draft: %w", name, err)
		}
		if receipt.Channel == "" {
			receipt.Channel = name
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func (m *Manager) SendFinalConfirmation(ctx context.Context, msg FinalConfirmationMessage) error {
	if m == nil {
		return nil
	}
	for name, ch := range m.channels {
		if err := ch.SendFinalConfirmation(ctx, msg); err != nil {
			return fmt.Errorf("channel %s final confirmation: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) VerifyInbound(channelName string, token string, msg InboundMessage) (Config, error) {
	if m == nil {
		return Config{}, fmt.Errorf("channels are not configured")
	}
	channelName = normalizeName(firstNonEmpty(channelName, msg.Channel))
	cfg, ok := m.configs[channelName]
	if !ok {
		return Config{}, fmt.Errorf("channel %q is not configured", channelName)
	}
	if strings.TrimSpace(cfg.InboundToken) != "" && strings.TrimSpace(token) != strings.TrimSpace(cfg.InboundToken) {
		return Config{}, fmt.Errorf("invalid channel token")
	}
	if !senderAllowed(cfg.AuthorizedSenders, msg) {
		return Config{}, fmt.Errorf("sender is not authorized")
	}
	return cfg, nil
}

func (m *Manager) ConfigForProvider(provider string) (Config, bool) {
	if m == nil {
		return Config{}, false
	}
	provider = normalizeName(provider)
	for _, cfg := range m.configs {
		if normalizeName(cfg.Provider) == provider {
			return cfg, true
		}
	}
	return Config{}, false
}

func EvaluateInbound(msg InboundMessage) InboundResult {
	command, commandRunID, approved, report, request, findingID := parseCommand(msg.Text, msg.RunID)
	result := InboundResult{
		RunID:       firstNonEmpty(msg.RunID, commandRunID),
		Channel:     strings.TrimSpace(msg.Channel),
		Sender:      firstNonEmpty(msg.SenderID, msg.SenderAddress),
		Command:     command,
		Approved:    approved,
		FinalReport: report,
		Request:     request,
		FindingID:   findingID,
	}
	if result.RunID == "" {
		result.Reason = "missing run id"
		return result
	}
	if !approved {
		result.Reason = "message recorded without explicit approval command"
	}
	switch command {
	case "revise":
		result.Revised = request != ""
		if result.Revised {
			result.Reason = ""
		}
	case "suppress":
		result.Suppressed = findingID != "" && request != ""
		if result.Suppressed {
			result.Reason = ""
		}
	}
	return result
}

func parseCommand(text string, runID string) (string, string, bool, string, string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false, "", "", ""
	}
	lines := strings.Split(text, "\n")
	first := strings.TrimSpace(lines[0])
	fields := strings.Fields(first)
	if len(fields) == 0 {
		return "", "", false, "", "", ""
	}
	command := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if command != "approve" && command != "revise" && command != "suppress" {
		return command, "", false, "", "", ""
	}
	commandRunID := ""
	if len(fields) >= 2 {
		commandRunID = strings.TrimSpace(fields[1])
	}
	if commandRunID == "" || (strings.TrimSpace(runID) != "" && commandRunID != strings.TrimSpace(runID)) {
		return command, commandRunID, false, "", "", ""
	}
	body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	switch command {
	case "approve":
		return command, commandRunID, true, body, "", ""
	case "revise":
		return command, commandRunID, false, "", body, ""
	case "suppress":
		if len(fields) < 3 {
			return command, commandRunID, false, "", body, ""
		}
		return command, commandRunID, false, "", body, strings.TrimSpace(fields[2])
	default:
		return command, commandRunID, false, "", "", ""
	}
}

func RunIDFromCommand(text string) string {
	_, runID, _, _, _, _ := parseCommand(text, "")
	return runID
}

func Setting(settings map[string]string, key string) string {
	if settings == nil {
		return ""
	}
	return strings.TrimSpace(settings[key])
}

type TwilioWhatsAppChannel struct {
	cfg        Config
	httpClient *http.Client
}

func NewTwilioWhatsAppChannel(cfg Config) TwilioWhatsAppChannel {
	return TwilioWhatsAppChannel{cfg: cfg, httpClient: http.DefaultClient}
}

func (c TwilioWhatsAppChannel) Name() string {
	return c.cfg.Name
}

func (c TwilioWhatsAppChannel) SendDraft(ctx context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	sid := Setting(c.cfg.Settings, "account_sid")
	token := Setting(c.cfg.Settings, "auth_token")
	from := Setting(c.cfg.Settings, "from")
	to := firstNonEmpty(Setting(c.cfg.Settings, "to"), firstSender(c.cfg.AuthorizedSenders))
	if sid == "" || token == "" || from == "" || to == "" {
		return DeliveryReceipt{}, fmt.Errorf("twilio_whatsapp channel %s missing account_sid/auth_token/from/to", c.cfg.Name)
	}
	body := fmt.Sprintf("7review draft ready\nrun: %s\nchange: %s %s\n\n%s\n\nReply with /approve %s, /revise %s, or /suppress %s <finding_id>.",
		msg.RunID, msg.Provider, msg.ChangeID, msg.Summary, msg.RunID, msg.RunID, msg.RunID)
	form := url.Values{}
	form.Set("From", from)
	form.Set("To", to)
	if contentSID := Setting(c.cfg.Settings, "content_sid"); contentSID != "" {
		form.Set("ContentSid", contentSID)
		variables, err := json.Marshal(map[string]string{
			"1": msg.RunID,
			"2": strings.TrimSpace(msg.Provider + " " + msg.ChangeID),
			"3": msg.Summary,
			"4": msg.WebURL,
		})
		if err != nil {
			return DeliveryReceipt{}, err
		}
		form.Set("ContentVariables", string(variables))
	} else {
		form.Set("Body", body)
	}
	apiBase := firstNonEmpty(Setting(c.cfg.Settings, "api_base_url"), "https://api.twilio.com")
	endpoint := strings.TrimRight(apiBase, "/") + "/2010-04-01/Accounts/" + url.PathEscape(sid) + "/Messages.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return DeliveryReceipt{}, err
	}
	req.SetBasicAuth(sid, token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeliveryReceipt{}, fmt.Errorf("twilio_whatsapp send status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		SID string `json:"sid"`
		URI string `json:"uri"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return DeliveryReceipt{Channel: c.cfg.Name, ExternalID: parsed.SID, URL: parsed.URI}, nil
}

func (c TwilioWhatsAppChannel) SendFinalConfirmation(ctx context.Context, msg FinalConfirmationMessage) error {
	_, err := c.SendDraft(ctx, DraftMessage{RunID: msg.RunID, Summary: "final review published", DraftReport: msg.FinalReport})
	return err
}

func ParseTwilioWhatsAppInbound(channelName string, form url.Values) InboundMessage {
	body := strings.TrimSpace(form.Get("Body"))
	return InboundMessage{
		Channel:       channelName,
		ExternalID:    strings.TrimSpace(form.Get("MessageSid")),
		RunID:         RunIDFromCommand(body),
		SenderAddress: strings.TrimSpace(form.Get("From")),
		Text:          body,
		ReceivedAt:    time.Now().UTC(),
	}
}

func VerifyTwilioSignature(authToken string, webhookURL string, form url.Values, signature string) bool {
	authToken = strings.TrimSpace(authToken)
	signature = strings.TrimSpace(signature)
	webhookURL = strings.TrimSpace(webhookURL)
	if authToken == "" || webhookURL == "" || signature == "" {
		return false
	}
	keys := make([]string, 0, len(form))
	for key := range form {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var base strings.Builder
	base.WriteString(webhookURL)
	for _, key := range keys {
		values := append([]string(nil), form[key]...)
		sort.Strings(values)
		for _, value := range values {
			base.WriteString(key)
			base.WriteString(value)
		}
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(base.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

type SendGridEmailChannel struct {
	cfg        Config
	httpClient *http.Client
}

func NewSendGridEmailChannel(cfg Config) SendGridEmailChannel {
	return SendGridEmailChannel{cfg: cfg, httpClient: http.DefaultClient}
}

func (c SendGridEmailChannel) Name() string {
	return c.cfg.Name
}

func (c SendGridEmailChannel) SendDraft(ctx context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	apiKey := Setting(c.cfg.Settings, "api_key")
	from := Setting(c.cfg.Settings, "from_email")
	to := firstNonEmpty(Setting(c.cfg.Settings, "to"), firstSender(c.cfg.AuthorizedSenders))
	if apiKey == "" || from == "" || to == "" {
		return DeliveryReceipt{}, fmt.Errorf("sendgrid_email channel %s missing api_key/from_email/to", c.cfg.Name)
	}
	payload := map[string]any{
		"personalizations": []map[string]any{{"to": []map[string]string{{"email": to}}}},
		"from":             map[string]string{"email": from},
		"subject":          "7review draft awaiting approval: " + msg.RunID,
		"content": []map[string]string{{
			"type":  "text/plain",
			"value": emailDraftBody(msg),
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	apiBase := firstNonEmpty(Setting(c.cfg.Settings, "api_base_url"), "https://api.sendgrid.com")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiBase, "/")+"/v3/mail/send", bytes.NewReader(data))
	if err != nil {
		return DeliveryReceipt{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeliveryReceipt{}, fmt.Errorf("sendgrid_email send status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return DeliveryReceipt{Channel: c.cfg.Name, ExternalID: resp.Header.Get("X-Message-Id")}, nil
}

func (c SendGridEmailChannel) SendFinalConfirmation(ctx context.Context, msg FinalConfirmationMessage) error {
	_, err := c.SendDraft(ctx, DraftMessage{RunID: msg.RunID, Summary: "final review published", DraftReport: msg.FinalReport})
	return err
}

type MailgunEmailChannel struct {
	cfg        Config
	httpClient *http.Client
}

func NewMailgunEmailChannel(cfg Config) MailgunEmailChannel {
	return MailgunEmailChannel{cfg: cfg, httpClient: http.DefaultClient}
}

func (c MailgunEmailChannel) Name() string {
	return c.cfg.Name
}

func (c MailgunEmailChannel) SendDraft(ctx context.Context, msg DraftMessage) (DeliveryReceipt, error) {
	apiKey := Setting(c.cfg.Settings, "api_key")
	domain := Setting(c.cfg.Settings, "domain")
	from := Setting(c.cfg.Settings, "from_email")
	to := firstNonEmpty(Setting(c.cfg.Settings, "to"), firstSender(c.cfg.AuthorizedSenders))
	if apiKey == "" || domain == "" || from == "" || to == "" {
		return DeliveryReceipt{}, fmt.Errorf("mailgun_email channel %s missing api_key/domain/from_email/to", c.cfg.Name)
	}
	form := url.Values{}
	form.Set("from", from)
	form.Set("to", to)
	form.Set("subject", "7review draft awaiting approval: "+msg.RunID)
	form.Set("text", emailDraftBody(msg))
	apiBase := firstNonEmpty(Setting(c.cfg.Settings, "api_base_url"), "https://api.mailgun.net")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiBase, "/")+"/v3/"+url.PathEscape(domain)+"/messages", strings.NewReader(form.Encode()))
	if err != nil {
		return DeliveryReceipt{}, err
	}
	req.SetBasicAuth("api", apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeliveryReceipt{}, fmt.Errorf("mailgun_email send status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var parsed struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return DeliveryReceipt{Channel: c.cfg.Name, ExternalID: parsed.ID}, nil
}

func (c MailgunEmailChannel) SendFinalConfirmation(ctx context.Context, msg FinalConfirmationMessage) error {
	_, err := c.SendDraft(ctx, DraftMessage{RunID: msg.RunID, Summary: "final review published", DraftReport: msg.FinalReport})
	return err
}

func ParseSendGridInbound(channelName string, form url.Values) InboundMessage {
	text := firstNonEmpty(form.Get("text"), form.Get("html"), form.Get("subject"))
	return InboundMessage{
		Channel:       channelName,
		ExternalID:    strings.TrimSpace(form.Get("email")),
		RunID:         RunIDFromCommand(text),
		SenderAddress: normalizeEmailAddress(form.Get("from")),
		Text:          strings.TrimSpace(text),
		ReceivedAt:    time.Now().UTC(),
	}
}

func ParseMailgunInbound(channelName string, form url.Values) InboundMessage {
	text := firstNonEmpty(form.Get("stripped-text"), form.Get("body-plain"), form.Get("text"), form.Get("subject"))
	return InboundMessage{
		Channel:       channelName,
		ExternalID:    strings.TrimSpace(firstNonEmpty(form.Get("Message-Id"), form.Get("message-id"))),
		RunID:         RunIDFromCommand(text),
		SenderAddress: normalizeEmailAddress(firstNonEmpty(form.Get("sender"), form.Get("from"))),
		Text:          strings.TrimSpace(text),
		ReceivedAt:    time.Now().UTC(),
	}
}

func VerifyMailgunSignature(signingKey string, timestamp string, token string, signature string) bool {
	signingKey = strings.TrimSpace(signingKey)
	timestamp = strings.TrimSpace(timestamp)
	token = strings.TrimSpace(token)
	signature = strings.TrimSpace(signature)
	if signingKey == "" || timestamp == "" || token == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp + token))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(signature)))
}

func VerifySendGridSignature(publicKeyPEM string, timestamp string, body []byte, signature string) bool {
	publicKeyPEM = strings.TrimSpace(publicKeyPEM)
	timestamp = strings.TrimSpace(timestamp)
	signature = strings.TrimSpace(signature)
	if publicKeyPEM == "" || timestamp == "" || len(body) == 0 || signature == "" {
		return false
	}
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return false
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false
	}
	publicKey, ok := parsed.(*ecdsa.PublicKey)
	if !ok {
		return false
	}
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append([]byte(timestamp), body...))
	return ecdsa.VerifyASN1(publicKey, sum[:], sigBytes)
}

func emailDraftBody(msg DraftMessage) string {
	return fmt.Sprintf("7review draft ready\n\nrun: %s\nrepository: %s\nchange: %s\nurl: %s\nsummary: %s\n\n%s\n\nApprove explicitly with:\n/approve %s\n\nRequest changes with:\n/revise %s\n<instruction>\n\nSuppress a finding with:\n/suppress %s <finding_id>\n<reason>",
		msg.RunID, msg.Repository, msg.ChangeID, msg.WebURL, msg.Summary, msg.DraftReport, msg.RunID, msg.RunID, msg.RunID)
}

func firstSender(senders []string) string {
	if len(senders) == 0 {
		return ""
	}
	return strings.TrimSpace(senders[0])
}

func normalizeEmailAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil {
		return value
	}
	return addr.Address
}

func senderAllowed(allowed []string, msg InboundMessage) bool {
	if len(allowed) == 0 {
		return false
	}
	senders := []string{msg.SenderID, msg.SenderAddress}
	for _, allowedSender := range allowed {
		for _, sender := range senders {
			if strings.EqualFold(strings.TrimSpace(allowedSender), strings.TrimSpace(sender)) && strings.TrimSpace(sender) != "" {
				return true
			}
		}
	}
	return false
}

func cleanSenders(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
