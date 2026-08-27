package channel

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

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
	body := draftBody("7review draft ready", msg)
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
