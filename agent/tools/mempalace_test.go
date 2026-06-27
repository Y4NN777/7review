package tools

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Y4NN777/7review/agent/review"
)

func TestMemPalaceStore_RecallAndWrite(t *testing.T) {
	var wrote bool
	store := NewMemPalaceStore("http://mempalace", time.Second)
	store.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/recall":
			return jsonResponse(review.MemoryRecall{Conventions: []string{"conv"}, Decisions: []string{"decision"}}), nil
		case "/write":
			wrote = true
			return response(http.StatusNoContent, ""), nil
		default:
			return response(http.StatusNotFound, ""), nil
		}
	})}

	recall, err := store.Recall(context.Background(), review.Request{Title: "auth change", ChangedPaths: []string{"auth.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if recall.Conventions[0] != "conv" || recall.Decisions[0] != "decision" {
		t.Fatalf("unexpected recall: %#v", recall)
	}
	if err := store.Write(context.Background(), review.UpdateProposal{Conventions: []string{"final"}}); err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("expected write call")
	}
}

func TestMemPalaceStore_ProposeUpdateRequiresApproval(t *testing.T) {
	store := NewMemPalaceStore("http://mempalace", time.Second)
	_, err := store.ProposeUpdate(context.Background(), review.NewContext(review.Request{}))
	if err == nil {
		t.Fatal("expected approval error")
	}
}

func TestMemPalaceStore_ProposeUpdateUsesFinalOnly(t *testing.T) {
	store := NewMemPalaceStore("http://mempalace", time.Second)
	rc := review.NewContext(review.Request{})
	rc.HILApproved = true
	rc.FinalReport = "final report"
	rc.Findings = []review.Finding{{ID: "F1", Title: "accepted"}}
	rc.HILAddedNotes = []string{"human note"}

	proposal, err := store.ProposeUpdate(context.Background(), rc)
	if err != nil {
		t.Fatal(err)
	}
	if len(proposal.Conventions) != 2 || proposal.Decisions[0] != "human note" {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}
}
