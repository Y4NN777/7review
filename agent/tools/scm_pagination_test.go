package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/review"
)

func TestGitHubClientEnrichPagesFilesAndCommits(t *testing.T) {
	var filePages int
	var commitPages int
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/7":
			return testJSON(t, gitHubPR{Title: "PR"}), nil
		case r.URL.Path == "/repos/o/r/pulls/7/files":
			filePages++
			switch pageNumber(r) {
			case 1:
				resp := testJSON(t, []gitHubFile{{Filename: "a.go", Patch: "@@"}})
				resp.Header.Set("Link", fmt.Sprintf(`<%s/repos/o/r/pulls/7/files?per_page=100&page=2>; rel="next"`, serverURL(r)))
				return resp, nil
			case 2:
				return testJSON(t, []gitHubFile{{Filename: "b.go", Patch: "@@"}}), nil
			}
		case r.URL.Path == "/repos/o/r/pulls/7/commits":
			commitPages++
			switch pageNumber(r) {
			case 1:
				resp := testJSON(t, []gitHubCommit{{SHA: "1"}})
				resp.Header.Set("Link", fmt.Sprintf(`<%s/repos/o/r/pulls/7/commits?per_page=100&page=2>; rel="next"`, serverURL(r)))
				return resp, nil
			case 2:
				return testJSON(t, []gitHubCommit{{SHA: "2"}}), nil
			}
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	ctx, err := client.Enrich(context.Background(), review.Request{Provider: "github", Repository: "o/r", ProjectID: "o/r", MRIID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if filePages != 2 || commitPages != 2 {
		t.Fatalf("expected two pages each, files=%d commits=%d", filePages, commitPages)
	}
	if len(ctx.Files) != 2 || ctx.Files[0].NewPath != "a.go" || ctx.Files[1].NewPath != "b.go" {
		t.Fatalf("files were not paged: %#v", ctx.Files)
	}
	if len(ctx.Commits) != 2 || ctx.Commits[1].SHA != "2" {
		t.Fatalf("commits were not paged: %#v", ctx.Commits)
	}
}

func TestGitHubClientEnrichUsesChangeIDWhenMRIIDMissing(t *testing.T) {
	var pulledPath string
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/7":
			pulledPath = r.URL.Path
			return testJSON(t, gitHubPR{Title: "PR"}), nil
		case r.URL.Path == "/repos/o/r/pulls/7/files":
			return testJSON(t, []gitHubFile{{Filename: "a.go", Patch: "@@"}}), nil
		case r.URL.Path == "/repos/o/r/pulls/7/commits":
			return testJSON(t, []gitHubCommit{}), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	ctx, err := client.Enrich(context.Background(), review.Request{Provider: "github", ProjectID: "o/r", ChangeID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if pulledPath != "/repos/o/r/pulls/7" || ctx.MRIID != 7 || ctx.Repository != "o/r" {
		t.Fatalf("request was not normalized from change id: path=%q ctx=%#v", pulledPath, ctx)
	}
}

func TestGitHubClientPublishFindsBotCommentOnSecondPage(t *testing.T) {
	var patched bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodGet:
			switch pageNumber(r) {
			case 1:
				resp := testJSON(t, []gitHubComment{{ID: 1, Body: "old"}})
				resp.Header.Set("Link", fmt.Sprintf(`<%s/repos/o/r/issues/7/comments?per_page=100&page=2>; rel="next"`, serverURL(r)))
				return resp, nil
			case 2:
				return testJSON(t, []gitHubComment{{ID: 2, Body: reportWithBotMarker("old", "draft")}}), nil
			}
		case r.URL.Path == "/repos/o/r/issues/comments/2" && r.Method == http.MethodPatch:
			patched = true
			return testResponse(http.StatusNoContent, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	err := client.PublishDraft(context.Background(), &review.SCMContext{Provider: "github", Repository: "o/r", MRIID: 7}, "new report")
	if err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("expected second-page bot comment to be patched")
	}
}

func TestGitHubClientPublishFinalDoesNotOverwriteDraftComment(t *testing.T) {
	var patchedDraft bool
	var postedFinal bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodGet:
			return testJSON(t, []gitHubComment{{ID: 1, Body: reportWithBotMarker("draft", "draft")}}), nil
		case r.URL.Path == "/repos/o/r/issues/comments/1" && r.Method == http.MethodPatch:
			patchedDraft = true
			return testResponse(http.StatusNoContent, ""), nil
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodPost:
			postedFinal = true
			body := readBody(t, r)
			if !strings.Contains(body, "kind=final") || strings.Contains(body, "kind=draft") {
				t.Fatalf("final body missing final marker: %s", body)
			}
			return testResponse(http.StatusCreated, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	err := client.PublishFinal(context.Background(), &review.SCMContext{Provider: "github", Repository: "o/r", MRIID: 7}, "<!-- 7review:bot-report project=o/r change=7 -->\nfinal")
	if err != nil {
		t.Fatal(err)
	}
	if patchedDraft || !postedFinal {
		t.Fatalf("expected final post without draft patch, patchedDraft=%v postedFinal=%v", patchedDraft, postedFinal)
	}
}

func TestGitHubClientPublishUsesProjectIDWhenRepositoryMissing(t *testing.T) {
	var posted bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodGet:
			return testJSON(t, []gitHubComment{}), nil
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodPost:
			posted = true
			return testResponse(http.StatusCreated, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	if err := client.PublishDraft(context.Background(), &review.SCMContext{Provider: "github", ProjectID: "o/r", MRIID: 7}, "draft"); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Fatal("expected draft to be posted using project id")
	}
}

func TestGitHubClientPublishInlineDraftCreatesReviewComment(t *testing.T) {
	var posted bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/7/comments" && r.Method == http.MethodGet:
			return testJSON(t, []gitHubReviewComment{}), nil
		case r.URL.Path == "/repos/o/r/pulls/7/comments" && r.Method == http.MethodPost:
			posted = true
			body := readBody(t, r)
			for _, want := range []string{`"commit_id":"head"`, `"path":"new/name.go"`, `"line":12`, `"side":"RIGHT"`, "7review:inline-comment", "finding=F1"} {
				if !strings.Contains(body, want) {
					t.Fatalf("github inline payload missing %q: %s", want, body)
				}
			}
			return testJSON(t, gitHubReviewComment{ID: 9, HTMLURL: "https://github.test/comment/9"}), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	comment, err := client.PublishInlineDraft(context.Background(), &review.SCMContext{
		Provider:   "github",
		Repository: "o/r",
		MRIID:      7,
		DiffRefs:   review.DiffRefs{HeadSHA: "head"},
	}, review.InlineComment{FindingID: "F1", Path: "new/name.go", NewPath: "new/name.go", Line: 12, Side: "RIGHT", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if !posted || comment.Status != "published" || comment.ProviderID != "9" {
		t.Fatalf("expected published inline comment, posted=%v comment=%#v", posted, comment)
	}
}

func TestGitHubClientPublishInlineDraftSkipsExistingMarker(t *testing.T) {
	var posted bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/pulls/7/comments" && r.Method == http.MethodGet:
			return testJSON(t, []gitHubReviewComment{{ID: 4, Body: inlineCommentMarker("F1"), HTMLURL: "https://github.test/comment/4"}}), nil
		case r.URL.Path == "/repos/o/r/pulls/7/comments" && r.Method == http.MethodPost:
			posted = true
			return testResponse(http.StatusCreated, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	comment, err := client.PublishInlineDraft(context.Background(), &review.SCMContext{Provider: "github", Repository: "o/r", MRIID: 7}, review.InlineComment{FindingID: "F1", Path: "a.go", Line: 1})
	if err != nil {
		t.Fatal(err)
	}
	if posted || comment.ProviderID != "4" || comment.URL == "" {
		t.Fatalf("expected existing inline marker to be reused, posted=%v comment=%#v", posted, comment)
	}
}

func TestGitHubClientPublishDraftUpdatesLegacyMarker(t *testing.T) {
	var patched bool
	client := NewGitHubClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/repos/o/r/issues/7/comments" && r.Method == http.MethodGet:
			return testJSON(t, []gitHubComment{{ID: 1, Body: gitHubBotMarker}}), nil
		case r.URL.Path == "/repos/o/r/issues/comments/1" && r.Method == http.MethodPatch:
			patched = true
			if !strings.Contains(readBody(t, r), "kind=draft") {
				t.Fatal("patched draft did not include kind marker")
			}
			return testResponse(http.StatusNoContent, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	if err := client.PublishDraft(context.Background(), &review.SCMContext{Provider: "github", Repository: "o/r", MRIID: 7}, "draft"); err != nil {
		t.Fatal(err)
	}
	if !patched {
		t.Fatal("expected legacy draft marker to be patched")
	}
}

func TestGitLabClientEnrichPagesDiffsAndCommits(t *testing.T) {
	var diffPages int
	var commitPages int
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7":
			return testJSON(t, gitLabMR{Title: "MR"}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/diffs":
			diffPages++
			if pageNumber(r) == 1 {
				resp := testJSON(t, []gitLabDiff{{NewPath: "a.go", Diff: "@@"}})
				resp.Header.Set("X-Next-Page", "2")
				return resp, nil
			}
			return testJSON(t, []gitLabDiff{{NewPath: "b.go", Diff: "@@"}}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/commits":
			commitPages++
			if pageNumber(r) == 1 {
				resp := testJSON(t, []gitLabCommit{{ID: "1"}})
				resp.Header.Set("X-Next-Page", "2")
				return resp, nil
			}
			return testJSON(t, []gitLabCommit{{ID: "2"}}), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	ctx, err := client.Enrich(context.Background(), review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if diffPages != 2 || commitPages != 2 {
		t.Fatalf("expected two pages each, diffs=%d commits=%d", diffPages, commitPages)
	}
	if len(ctx.Files) != 2 || ctx.Files[1].NewPath != "b.go" {
		t.Fatalf("diffs were not paged: %#v", ctx.Files)
	}
	if len(ctx.Commits) != 2 || ctx.Commits[1].SHA != "2" {
		t.Fatalf("commits were not paged: %#v", ctx.Commits)
	}
}

func TestGitLabClientEnrichFallsBackToChangesWhenDiffsFail(t *testing.T) {
	var usedChanges bool
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7":
			return testJSON(t, gitLabMR{Title: "MR"}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/diffs":
			return testResponse(http.StatusInternalServerError, `{"message":"500 Internal Server Error"}`), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/changes":
			usedChanges = true
			return testJSON(t, gitLabChanges{Changes: []gitLabDiff{{NewPath: "fallback.go", Diff: "@@"}}}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/commits":
			return testJSON(t, []gitLabCommit{{ID: "1"}}), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	ctx, err := client.Enrich(context.Background(), review.Request{Provider: "gitlab", ProjectID: "p", MRIID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if !usedChanges {
		t.Fatal("expected GitLab changes fallback to be used")
	}
	if len(ctx.Files) != 1 || ctx.Files[0].NewPath != "fallback.go" {
		t.Fatalf("expected fallback diff in context: %#v", ctx.Files)
	}
}

func TestGitLabClientPublishFindsBotNoteOnSecondPage(t *testing.T) {
	var updated bool
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/notes" && r.Method == http.MethodGet:
			if pageNumber(r) == 1 {
				resp := testJSON(t, []gitLabNote{{ID: 1, Body: "old"}})
				resp.Header.Set("X-Next-Page", "2")
				return resp, nil
			}
			return testJSON(t, []gitLabNote{{ID: 2, Body: reportWithBotMarker("old", "draft")}}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/notes/2" && r.Method == http.MethodPut:
			updated = true
			return testResponse(http.StatusNoContent, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	err := client.PublishDraft(context.Background(), &review.SCMContext{Provider: "gitlab", ProjectID: "p", MRIID: 7}, "new report")
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected second-page bot note to be updated")
	}
}

func TestGitLabClientPublishFinalDoesNotOverwriteDraftNote(t *testing.T) {
	var updatedDraft bool
	var postedFinal bool
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/notes" && r.Method == http.MethodGet:
			return testJSON(t, []gitLabNote{{ID: 1, Body: reportWithBotMarker("draft", "draft")}}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/notes/1" && r.Method == http.MethodPut:
			updatedDraft = true
			return testResponse(http.StatusNoContent, ""), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/notes" && r.Method == http.MethodPost:
			postedFinal = true
			body := readBody(t, r)
			if !strings.Contains(body, "kind=final") || strings.Contains(body, "kind=draft") {
				t.Fatalf("final body missing final marker: %s", body)
			}
			return testResponse(http.StatusCreated, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	err := client.PublishFinal(context.Background(), &review.SCMContext{Provider: "gitlab", ProjectID: "p", MRIID: 7}, "<!-- 7review:bot-report project=p change=7 -->\nfinal")
	if err != nil {
		t.Fatal(err)
	}
	if updatedDraft || !postedFinal {
		t.Fatalf("expected final post without draft update, updatedDraft=%v postedFinal=%v", updatedDraft, postedFinal)
	}
}

func TestGitLabClientPublishInlineDraftCreatesDiscussion(t *testing.T) {
	var posted bool
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/discussions" && r.Method == http.MethodGet:
			return testJSON(t, []gitLabDiscussion{}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/discussions" && r.Method == http.MethodPost:
			posted = true
			body := readBody(t, r)
			for _, want := range []string{`"base_sha":"base"`, `"start_sha":"start"`, `"head_sha":"head"`, `"old_path":"old/name.go"`, `"new_path":"new/name.go"`, `"new_line":12`, "7review:inline-comment", "finding=F1"} {
				if !strings.Contains(body, want) {
					t.Fatalf("gitlab inline payload missing %q: %s", want, body)
				}
			}
			return testJSON(t, gitLabDiscussion{ID: "discussion-1", Notes: []gitLabNote{{ID: 11, URL: "https://gitlab.test/note/11"}}}), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	comment, err := client.PublishInlineDraft(context.Background(), &review.SCMContext{
		Provider:  "gitlab",
		ProjectID: "p",
		MRIID:     7,
		DiffRefs:  review.DiffRefs{BaseSHA: "base", StartSHA: "start", HeadSHA: "head"},
	}, review.InlineComment{FindingID: "F1", Path: "new/name.go", OldPath: "old/name.go", NewPath: "new/name.go", Line: 12, Side: "RIGHT", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if !posted || comment.Status != "published" || comment.ProviderID != "11" {
		t.Fatalf("expected published gitlab inline comment, posted=%v comment=%#v", posted, comment)
	}
}

func TestGitLabClientPublishInlineDraftSkipsExistingMarker(t *testing.T) {
	var posted bool
	client := NewGitLabClient("http://agent.test", "token")
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/discussions" && r.Method == http.MethodGet:
			return testJSON(t, []gitLabDiscussion{{ID: "d", Notes: []gitLabNote{{ID: 5, Body: inlineCommentMarker("F1"), URL: "https://gitlab.test/note/5"}}}}), nil
		case r.URL.Path == "/api/v4/projects/p/merge_requests/7/discussions" && r.Method == http.MethodPost:
			posted = true
			return testResponse(http.StatusCreated, ""), nil
		}
		return testResponse(http.StatusNotFound, "not found"), nil
	})}

	comment, err := client.PublishInlineDraft(context.Background(), &review.SCMContext{Provider: "gitlab", ProjectID: "p", MRIID: 7}, review.InlineComment{FindingID: "F1", Path: "a.go", Line: 1})
	if err != nil {
		t.Fatal(err)
	}
	if posted || comment.ProviderID != "5" || comment.URL == "" {
		t.Fatalf("expected existing gitlab inline marker to be reused, posted=%v comment=%#v", posted, comment)
	}
}

func testJSON(t *testing.T, value any) *http.Response {
	t.Helper()
	var b strings.Builder
	if err := json.NewEncoder(&b).Encode(value); err != nil {
		t.Fatal(err)
	}
	resp := testResponse(http.StatusOK, b.String())
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func pageNumber(r *http.Request) int {
	page := r.URL.Query().Get("page")
	if page == "" {
		return 1
	}
	n, err := strconv.Atoi(page)
	if err != nil {
		return 1
	}
	return n
}

func serverURL(r *http.Request) string {
	return r.URL.Scheme + "://" + r.URL.Host
}
