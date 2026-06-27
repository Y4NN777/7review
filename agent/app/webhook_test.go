package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/review"
)

func TestGitLabWebhookIgnoresNonMergeRequestEvents(t *testing.T) {
	called := false
	handler := gitLabWebhookHandler("secret", func(review.Request) error {
		called = true
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(`{
		"object_kind":"note",
		"event_type":"note",
		"project":{"id":1},
		"object_attributes":{"iid":7,"action":"update"}
	}`))
	req.Header.Set("X-Gitlab-Token", "secret")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("handler should not run for non-MR events")
	}
}

func TestGitLabWebhookNormalizesReviewableEventMetadata(t *testing.T) {
	var got review.Request
	handler := gitLabWebhookHandler("secret", func(req review.Request) error {
		got = req
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(`{
		"object_kind":"merge_request",
		"event_type":"merge_request",
		"project":{"id":42},
		"object_attributes":{
			"iid":7,
			"action":"update",
			"title":"Fix auth",
			"description":"body",
			"url":"https://gitlab.example.com/p/-/merge_requests/7",
			"source_branch":"feature",
			"target_branch":"main",
			"last_commit":{"id":"abc"},
			"labels":["security"]
		},
		"user":{"username":"alice"}
	}`))
	req.Header.Set("X-Gitlab-Token", "secret")
	req.Header.Set("X-Gitlab-Event-UUID", "delivery-1")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if got.Provider != "gitlab" || got.ProjectID != "42" || got.MRIID != 7 || got.DeliveryID != "delivery-1" || got.EventAction != "update" {
		t.Fatalf("request not normalized: %#v", got)
	}
	if got.Title != "Fix auth" || got.WebURL == "" || got.SourceSHA != "abc" || got.Author != "alice" {
		t.Fatalf("request metadata missing: %#v", got)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "security" {
		t.Fatalf("labels not normalized: %#v", got.Labels)
	}
}

func TestGitLabWebhookNormalizesLabelPayloadVariants(t *testing.T) {
	var got review.Request
	handler := gitLabWebhookHandler("secret", func(req review.Request) error {
		got = req
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(`{
		"object_kind":"merge_request",
		"event_type":"merge_request",
		"labels":[{"title":"security"},{"name":"backend"}],
		"project":{"id":42},
		"object_attributes":{
			"iid":7,
			"action":"unapproval",
			"title":"Fix auth",
			"labels":["security"," api ",""]
		},
		"user":{"username":"alice"}
	}`))
	req.Header.Set("X-Gitlab-Token", "secret")
	req.Header.Set("X-Gitlab-Delivery", "delivery-fallback")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if got.DeliveryID != "delivery-fallback" || got.EventAction != "unapproval" {
		t.Fatalf("request metadata not normalized: %#v", got)
	}
	want := []string{"security", "api", "backend"}
	if strings.Join(got.Labels, ",") != strings.Join(want, ",") {
		t.Fatalf("labels not normalized: got %#v want %#v", got.Labels, want)
	}
}

func TestGitLabWebhookRejectsOversizedPayload(t *testing.T) {
	handler := gitLabWebhookHandler("", func(review.Request) error {
		t.Fatal("handler should not run for oversized payload")
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/gitlab", strings.NewReader(strings.Repeat("x", int(webhookMaxBodyBytes)+1)))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubWebhookNormalizesDeliveryAndAction(t *testing.T) {
	body := `{
		"action":"synchronize",
		"number":7,
		"repository":{"full_name":"o/r"},
		"pull_request":{
			"title":"Fix auth",
			"body":"body",
			"html_url":"https://github.com/o/r/pull/7",
			"user":{"login":"alice"},
			"head":{"ref":"feature","sha":"abc"},
			"base":{"ref":"main","sha":"def"},
			"labels":[{"name":"security"}]
		}
	}`
	var got review.Request
	handler := gitHubWebhookHandler("secret", func(req review.Request) error {
		got = req
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-2")
	req.Header.Set("X-Hub-Signature-256", githubSignature("secret", body))
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	if got.Provider != "github" || got.DeliveryID != "delivery-2" || got.EventAction != "synchronize" || got.ProjectID != "o/r" {
		t.Fatalf("request not normalized: %#v", got)
	}
	if got.Repository != "o/r" || got.MRIID != 7 || got.ChangeID != "7" || got.SourceSHA != "abc" || got.TargetSHA != "def" {
		t.Fatalf("pull request identity not normalized: %#v", got)
	}
}

func TestGitHubWebhookRejectsOversizedPayload(t *testing.T) {
	handler := gitHubWebhookHandler("", func(review.Request) error {
		t.Fatal("handler should not run for oversized payload")
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(strings.Repeat("x", int(webhookMaxBodyBytes)+1)))
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func githubSignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
