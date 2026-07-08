package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/channel"
	"github.com/Y4NN777/7review/agent/pipeline"
	"github.com/Y4NN777/7review/agent/review"
)

func TestHandleChannelInboundRecordsMessageWithoutImplicitApproval(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:     store,
			Channels: channel.NewManager([]channel.Config{{Name: "ops", Provider: "log", Enabled: true, InboundToken: "secret", AuthorizedSenders: []string{"operator"}}}),
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/channels/ops/inbound", strings.NewReader(`{"run_id":"`+run.ID+`","sender_id":"operator","text":"looks good"}`))
	req.Header.Set("X-7Review-Channel-Token", "secret")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 0 {
		t.Fatalf("ambiguous message should not enqueue approval")
	}
	stored, err := store.Get(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !eventTypeSeen(stored.Events, "channel_message_received") {
		t.Fatalf("channel event not recorded: %#v", stored.Events)
	}
}

func TestHandleChannelInboundRejectsUnauthorizedSender(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:     pipeline.NewMemoryRunStore(),
			Channels: channel.NewManager([]channel.Config{{Name: "ops", Provider: "log", Enabled: true, InboundToken: "secret", AuthorizedSenders: []string{"operator"}}}),
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/channels/ops/inbound", strings.NewReader(`{"run_id":"owner/repo!7","sender_id":"attacker","text":"/approve owner/repo!7"}`))
	req.Header.Set("X-7Review-Channel-Token", "secret")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleChannelInboundEnqueuesExplicitApproval(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:     store,
			Channels: channel.NewManager([]channel.Config{{Name: "ops", Provider: "log", Enabled: true, InboundToken: "secret", AuthorizedSenders: []string{"operator"}}}),
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/channels/ops/inbound", strings.NewReader(`{"run_id":"`+run.ID+`","sender_id":"operator","text":"/approve `+run.ID+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatalf("explicit approval should enqueue approval work, queued=%d", len(s.work))
	}
}

func TestHandleChannelInboundEnqueuesDraftRevision(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs:     store,
			Channels: channel.NewManager([]channel.Config{{Name: "ops", Provider: "log", Enabled: true, InboundToken: "secret", AuthorizedSenders: []string{"operator"}}}),
		},
		work: make(chan workItem, 1),
	}
	req := httptest.NewRequest(http.MethodPost, "/channels/ops/inbound", strings.NewReader(`{"run_id":"`+run.ID+`","sender_id":"operator","text":"/revise `+run.ID+`\nFocus on auth only"}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatalf("revision should enqueue work, queued=%d", len(s.work))
	}
}

func TestHandleTwilioWhatsAppInboundEnqueuesExplicitApproval(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	webhookURL := "https://7review.example.com/channels/twilio/whatsapp"
	authToken := "twilio-secret"
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs: store,
			Channels: channel.NewManager([]channel.Config{{
				Name:              "operator_whatsapp",
				Provider:          "twilio_whatsapp",
				Enabled:           true,
				AuthorizedSenders: []string{"whatsapp:+33600000000"},
				Settings:          map[string]string{"auth_token": authToken, "webhook_url": webhookURL},
			}}),
		},
		work: make(chan workItem, 1),
	}
	form := url.Values{}
	form.Set("From", "whatsapp:+33600000000")
	form.Set("To", "whatsapp:+14155238886")
	form.Set("Body", "/approve "+run.ID)
	form.Set("MessageSid", "SM123")
	req := httptest.NewRequest(http.MethodPost, "/channels/twilio/whatsapp", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", signTwilio(authToken, webhookURL, form))
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatalf("twilio approval should enqueue work, queued=%d", len(s.work))
	}
}

func TestHandleTwilioWhatsAppInboundRejectsBadSignature(t *testing.T) {
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs: pipeline.NewMemoryRunStore(),
			Channels: channel.NewManager([]channel.Config{{
				Name:              "operator_whatsapp",
				Provider:          "twilio_whatsapp",
				Enabled:           true,
				AuthorizedSenders: []string{"whatsapp:+33600000000"},
				Settings:          map[string]string{"auth_token": "twilio-secret", "webhook_url": "https://7review.example.com/channels/twilio/whatsapp"},
			}}),
		},
		work: make(chan workItem, 1),
	}
	form := url.Values{}
	form.Set("From", "whatsapp:+33600000000")
	form.Set("Body", "/approve owner/repo!7")
	req := httptest.NewRequest(http.MethodPost, "/channels/twilio/whatsapp", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Twilio-Signature", "bad")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSendGridInboundEnqueuesRevision(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs: store,
			Channels: channel.NewManager([]channel.Config{{
				Name:              "operator_email",
				Provider:          "sendgrid_email",
				Enabled:           true,
				InboundToken:      "sendgrid-oauth",
				AuthorizedSenders: []string{"operator@example.com"},
			}}),
		},
		work: make(chan workItem, 1),
	}
	form := url.Values{}
	form.Set("from", "operator@example.com")
	form.Set("text", "/revise "+run.ID+"\nFocus on auth only")
	req := httptest.NewRequest(http.MethodPost, "/channels/sendgrid/inbound", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer sendgrid-oauth")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatalf("sendgrid revision should enqueue work, queued=%d", len(s.work))
	}
}

func TestHandleMailgunInboundEnqueuesSuppression(t *testing.T) {
	store := pipeline.NewMemoryRunStore()
	run, err := store.Start(context.Background(), review.Request{Provider: "github", ProjectID: "owner/repo", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	signingKey := "mailgun-secret"
	s := &Server{
		pipeline: &pipeline.Pipeline{
			Jobs: store,
			Channels: channel.NewManager([]channel.Config{{
				Name:              "operator_email",
				Provider:          "mailgun_email",
				Enabled:           true,
				AuthorizedSenders: []string{"operator@example.com"},
				Settings:          map[string]string{"signing_key": signingKey},
			}}),
		},
		work: make(chan workItem, 1),
	}
	form := url.Values{}
	form.Set("sender", "operator@example.com")
	form.Set("stripped-text", "/suppress "+run.ID+" F1\nfalse positive")
	form.Set("timestamp", "1700000000")
	form.Set("token", "random-token")
	form.Set("signature", signMailgun(signingKey, form.Get("timestamp"), form.Get("token")))
	req := httptest.NewRequest(http.MethodPost, "/channels/mailgun/inbound", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	s.handleChannelInbound(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(s.work) != 1 {
		t.Fatalf("mailgun suppression should enqueue work, queued=%d", len(s.work))
	}
}

func signTwilio(authToken string, webhookURL string, form url.Values) string {
	keys := make([]string, 0, len(form))
	for key := range form {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	base := webhookURL
	for _, key := range keys {
		base += key + form.Get(key)
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(base))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func signMailgun(signingKey string, timestamp string, token string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp + token))
	return hex.EncodeToString(mac.Sum(nil))
}

func eventTypeSeen(events []pipeline.RunEvent, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
