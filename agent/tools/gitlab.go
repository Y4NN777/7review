package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
)

type GitLabClient struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

func NewGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTPClient: http.DefaultClient,
	}
}

func (c *GitLabClient) Enrich(ctx context.Context, req review.Request) (*review.SCMContext, error) {
	if c == nil || c.BaseURL == "" || c.Token == "" {
		return NoopSCM{}.Enrich(ctx, req)
	}
	projectID := req.ProjectID
	mrIID := req.MRIID

	var mr gitLabMR
	if err := c.get(ctx, fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d", url.PathEscape(projectID), mrIID), &mr); err != nil {
		return nil, err
	}

	var diffs []gitLabDiff
	if err := c.getAll(ctx, fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/diffs?per_page=100", url.PathEscape(projectID), mrIID), &diffs); err != nil {
		return nil, err
	}

	var commits []gitLabCommit
	_ = c.getAll(ctx, fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/commits?per_page=100", url.PathEscape(projectID), mrIID), &commits)

	files := make([]review.ChangedFile, 0, len(diffs))
	paths := make([]string, 0, len(diffs))
	for _, diff := range diffs {
		status := "modified"
		switch {
		case diff.NewFile:
			status = "added"
		case diff.DeletedFile:
			status = "deleted"
		case diff.RenamedFile:
			status = "renamed"
		}
		files = append(files, review.ChangedFile{
			OldPath: diff.OldPath,
			NewPath: diff.NewPath,
			Patch:   diff.Diff,
			Status:  status,
		})
		if diff.NewPath != "" {
			paths = append(paths, diff.NewPath)
		}
	}

	normalizedCommits := make([]review.Commit, 0, len(commits))
	for _, commit := range commits {
		normalizedCommits = append(normalizedCommits, review.Commit{
			SHA:     commit.ID,
			Title:   commit.Title,
			Message: commit.Message,
			Author:  commit.AuthorName,
		})
	}

	return &review.SCMContext{
		Provider:    "gitlab",
		ProjectID:   projectID,
		Repository:  req.Repository,
		ChangeID:    strconv.Itoa(mrIID),
		MRIID:       mrIID,
		Title:       firstNonEmpty(mr.Title, req.Title),
		Description: firstNonEmpty(mr.Description, req.Description),
		Author:      firstNonEmpty(mr.Author.Username, req.Author),
		WebURL:      mr.WebURL,
		Labels:      append([]string(nil), mr.Labels...),
		DiffRefs: review.DiffRefs{
			BaseSHA:  firstNonEmpty(mr.DiffRefs.BaseSHA, req.TargetSHA),
			HeadSHA:  firstNonEmpty(mr.DiffRefs.HeadSHA, req.SourceSHA),
			StartSHA: mr.DiffRefs.StartSHA,
		},
		Commits: normalizedCommits,
		Files:   files,
	}, nil
}

func (c *GitLabClient) PublishDraft(ctx context.Context, source *review.SCMContext, report string) error {
	return c.upsertNote(ctx, source, report, "draft")
}

func (c *GitLabClient) PublishFinal(ctx context.Context, source *review.SCMContext, report string) error {
	return c.upsertNote(ctx, source, report, "final")
}

func (c *GitLabClient) upsertNote(ctx context.Context, source *review.SCMContext, body string, kind string) error {
	if c == nil || c.BaseURL == "" || c.Token == "" || source == nil {
		return nil
	}
	body = reportWithBotMarker(body, kind)
	notesPath := fmt.Sprintf("/api/v4/projects/%s/merge_requests/%d/notes", url.PathEscape(source.ProjectID), source.MRIID)
	var notes []gitLabNote
	if err := c.getAll(ctx, notesPath+"?per_page=100", &notes); err != nil {
		return err
	}
	for _, note := range notes {
		if hasBotMarkerKind(note.Body, kind) || (kind == "draft" && hasLegacyBotMarker(note.Body)) {
			return c.send(ctx, http.MethodPut, fmt.Sprintf("%s/%d", notesPath, note.ID), map[string]string{"body": body}, nil)
		}
	}
	return c.send(ctx, http.MethodPost, notesPath, map[string]string{"body": body}, nil)
}

func (c *GitLabClient) get(ctx context.Context, path string, out any) error {
	return c.send(ctx, http.MethodGet, path, nil, out)
}

func (c *GitLabClient) getAll(ctx context.Context, path string, out any) error {
	return fetchPages(ctx, path, out, c.sendPage, nextGitLabPage)
}

func (c *GitLabClient) send(ctx context.Context, method, path string, in any, out any) error {
	_, err := c.sendPage(ctx, method, path, in, out)
	return err
}

func (c *GitLabClient) sendPage(ctx context.Context, method, path string, in any, out any) (http.Header, error) {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("gitlab: %s %s: %s: %s", method, path, resp.Status, readToolErrorBody(resp.Body))
	}
	if out == nil {
		return resp.Header.Clone(), nil
	}
	return resp.Header.Clone(), decodeToolJSON("gitlab", method, path, resp.Body, out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type gitLabMR struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	WebURL      string   `json:"web_url"`
	Labels      []string `json:"labels"`
	Author      struct {
		Username string `json:"username"`
	} `json:"author"`
	DiffRefs struct {
		BaseSHA  string `json:"base_sha"`
		HeadSHA  string `json:"head_sha"`
		StartSHA string `json:"start_sha"`
	} `json:"diff_refs"`
}

type gitLabDiff struct {
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
	Diff        string `json:"diff"`
	NewFile     bool   `json:"new_file"`
	RenamedFile bool   `json:"renamed_file"`
	DeletedFile bool   `json:"deleted_file"`
}

type gitLabCommit struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	AuthorName string `json:"author_name"`
}

type gitLabNote struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}
