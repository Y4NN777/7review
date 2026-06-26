package channels

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// MergeRequestHandler is called with the GitLab project ID and MR IID.
type MergeRequestHandler func(projectID string, mrIID int)

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
				IID int `json:"iid"`
			} `json:"object_attributes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, "invalid webhook payload", http.StatusBadRequest)
			return
		}
		if event.Project.ID == 0 || event.ObjectAttributes.IID == 0 {
			http.Error(w, "missing project or merge request id", http.StatusBadRequest)
			return
		}

		go handler(stringInt(event.Project.ID), event.ObjectAttributes.IID)
		w.WriteHeader(http.StatusAccepted)
	}
}

func stringInt(n int) string {
	return strconv.Itoa(n)
}
