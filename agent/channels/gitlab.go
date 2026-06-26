package channels

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Y4NN777/7review/agent/review"
)

// MergeRequestHandler is called with a normalized merge request review request.
type MergeRequestHandler func(review.Request)

// GitLabWebhookHandler validates GitLab webhook requests and dispatches MR jobs.
func GitLabWebhookHandler(secret string, handler MergeRequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secret != "" && r.Header.Get("X-Gitlab-Token") != secret {
			http.Error(w, "invalid webhook token", http.StatusUnauthorized)
			return
		}

		var event struct {
			Project struct {
				ID int `json:"id"`
			} `json:"project"`
			ObjectAttributes struct {
				IID          int    `json:"iid"`
				SourceBranch string `json:"source_branch"`
				TargetBranch string `json:"target_branch"`
				LastCommit   struct {
					ID string `json:"id"`
				} `json:"last_commit"`
				Labels []string `json:"labels"`
			} `json:"object_attributes"`
			User struct {
				Username string `json:"username"`
			} `json:"user"`
			Changes struct {
				UpdatedByID any `json:"updated_by_id"`
			} `json:"changes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid webhook payload", http.StatusBadRequest)
			return
		}
		if event.Project.ID == 0 || event.ObjectAttributes.IID == 0 {
			http.Error(w, "missing project or merge request id", http.StatusBadRequest)
			return
		}

		go handler(review.Request{
			ProjectID:    stringInt(event.Project.ID),
			MRIID:        event.ObjectAttributes.IID,
			SourceSHA:    event.ObjectAttributes.LastCommit.ID,
			SourceBranch: event.ObjectAttributes.SourceBranch,
			TargetBranch: event.ObjectAttributes.TargetBranch,
			Author:       event.User.Username,
			Labels:       event.ObjectAttributes.Labels,
		})
		w.WriteHeader(http.StatusAccepted)
	}
}

func stringInt(n int) string {
	return strconv.Itoa(n)
}
