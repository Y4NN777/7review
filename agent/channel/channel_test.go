package channel

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"net/url"
	"testing"
)

func TestManagerVerifyInboundRequiresTokenAndAuthorizedSender(t *testing.T) {
	manager := NewManager([]Config{{
		Name:              "ops",
		Provider:          "log",
		Enabled:           true,
		InboundToken:      "secret",
		AuthorizedSenders: []string{"operator@example.com"},
	}})
	msg := InboundMessage{Channel: "ops", RunID: "owner/repo!7", SenderAddress: "operator@example.com", Text: "/approve owner/repo!7"}
	if _, err := manager.VerifyInbound("ops", "secret", msg); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.VerifyInbound("ops", "wrong", msg); err == nil {
		t.Fatal("expected invalid token error")
	}
	msg.SenderAddress = "attacker@example.com"
	if _, err := manager.VerifyInbound("ops", "secret", msg); err == nil {
		t.Fatal("expected unauthorized sender error")
	}
}

func TestEvaluateInboundOnlyApprovesExplicitRunScopedCommand(t *testing.T) {
	msg := InboundMessage{RunID: "owner/repo!7", SenderID: "operator", Text: "looks good"}
	result := EvaluateInbound(msg)
	if result.Approved || result.Reason == "" {
		t.Fatalf("ambiguous message should not approve: %#v", result)
	}
	msg.Text = "/approve owner/repo!8"
	result = EvaluateInbound(msg)
	if result.Approved {
		t.Fatalf("wrong run id should not approve: %#v", result)
	}
	msg.Text = "/approve owner/repo!7\napproved final text"
	result = EvaluateInbound(msg)
	if !result.Approved || result.FinalReport != "approved final text" {
		t.Fatalf("explicit approval not detected: %#v", result)
	}
}

func TestEvaluateInboundExtractsRunIDFromProviderText(t *testing.T) {
	msg := InboundMessage{Channel: "operator_whatsapp", SenderAddress: "whatsapp:+33600000000", Text: "/approve owner/repo!7"}
	result := EvaluateInbound(msg)
	if !result.Approved || result.RunID != "owner/repo!7" {
		t.Fatalf("provider command run id not extracted: %#v", result)
	}
}

func TestEvaluateInboundSupportsDraftRevisionAndSuppressionCommands(t *testing.T) {
	msg := InboundMessage{RunID: "owner/repo!7", SenderID: "operator", Text: "/revise owner/repo!7\nFocus on auth only"}
	result := EvaluateInbound(msg)
	if !result.Revised || result.Request != "Focus on auth only" || result.Approved {
		t.Fatalf("revision command not detected: %#v", result)
	}
	msg.Text = "/suppress owner/repo!7 F1\nFalse positive because middleware already checks this"
	result = EvaluateInbound(msg)
	if !result.Suppressed || result.FindingID != "F1" || result.Request == "" || result.Approved {
		t.Fatalf("suppression command not detected: %#v", result)
	}
}

func TestVerifyTwilioSignature(t *testing.T) {
	form := url.Values{}
	form.Set("From", "whatsapp:+33600000000")
	form.Set("To", "whatsapp:+14155238886")
	form.Set("Body", "/approve owner/repo!7")
	webhookURL := "https://7review.example.com/channels/twilio/whatsapp"
	authToken := "twilio-secret"
	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(webhookURL + "Body" + form.Get("Body") + "From" + form.Get("From") + "To" + form.Get("To")))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !VerifyTwilioSignature(authToken, webhookURL, form, signature) {
		t.Fatal("expected valid twilio signature")
	}
	if VerifyTwilioSignature(authToken, webhookURL, form, "bad") {
		t.Fatal("expected invalid twilio signature")
	}
}

func TestParseTwilioWhatsAppInbound(t *testing.T) {
	form := url.Values{}
	form.Set("From", "whatsapp:+33600000000")
	form.Set("Body", "/revise owner/repo!7\nFocus on auth")
	form.Set("MessageSid", "SM123")
	msg := ParseTwilioWhatsAppInbound("operator_whatsapp", form)
	if msg.RunID != "owner/repo!7" || msg.SenderAddress != "whatsapp:+33600000000" || msg.ExternalID != "SM123" {
		t.Fatalf("twilio inbound not parsed: %#v", msg)
	}
}

func TestVerifyMailgunSignature(t *testing.T) {
	signingKey := "mailgun-secret"
	timestamp := "1700000000"
	token := "random-token"
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp + token))
	signature := hex.EncodeToString(mac.Sum(nil))
	if !VerifyMailgunSignature(signingKey, timestamp, token, signature) {
		t.Fatal("expected valid mailgun signature")
	}
	if VerifyMailgunSignature(signingKey, timestamp, token, "bad") {
		t.Fatal("expected invalid mailgun signature")
	}
}

func TestVerifySendGridSignature(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	timestamp := "1700000000"
	body := []byte("from=operator%40example.com&text=%2Fapprove+owner%2Frepo%217")
	sum := sha256.Sum256(append([]byte(timestamp), body...))
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	encodedSignature := base64.StdEncoding.EncodeToString(signature)
	if !VerifySendGridSignature(publicPEM, timestamp, body, encodedSignature) {
		t.Fatal("expected valid sendgrid signature")
	}
	if VerifySendGridSignature(publicPEM, timestamp, body, "bad") {
		t.Fatal("expected invalid sendgrid signature")
	}
}

func TestParseEmailInbound(t *testing.T) {
	sendgrid := url.Values{}
	sendgrid.Set("from", "Operator <operator@example.com>")
	sendgrid.Set("text", "/approve owner/repo!7")
	msg := ParseSendGridInbound("operator_email", sendgrid)
	if msg.RunID != "owner/repo!7" || msg.SenderAddress != "operator@example.com" {
		t.Fatalf("sendgrid inbound not parsed: %#v", msg)
	}
	mailgun := url.Values{}
	mailgun.Set("sender", "operator@example.com")
	mailgun.Set("stripped-text", "/suppress owner/repo!7 F1\nfalse positive")
	msg = ParseMailgunInbound("operator_email", mailgun)
	if msg.RunID != "owner/repo!7" || msg.SenderAddress != "operator@example.com" {
		t.Fatalf("mailgun inbound not parsed: %#v", msg)
	}
}
