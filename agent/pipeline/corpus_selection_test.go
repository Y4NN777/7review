package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Y4NN777/7review/agent/review"
)

func TestExtractReviewSignals_IdentifiersRoutesPathsAndEntities(t *testing.T) {
	req := review.Request{
		Title:        "Implement OPC-DELALL for INV-9",
		Description:  "Touches FR-12 and /messages/{message_id}",
		Labels:       []string{"backend", "contract"},
		ChangedPaths: []string{"services/backend/messages/delete.py"},
	}
	diff := &review.StructuredDiff{Files: []review.FileDiff{{
		Path:  "services/backend/messages/delete.py",
		Patch: "+DELETE FROM messages WHERE id = message_id\n+class DeleteMessageHandler:\n+  route = \"/messages/{message_id}\"",
	}}}

	signals := extractReviewSignals(req, diff, nil, []review.Section{{Title: "api-contract-review"}})

	for _, want := range []string{"OPC-DELALL", "INV-9", "FR-12"} {
		if _, ok := signals.IDs[want]; !ok {
			t.Fatalf("missing id %s in %#v", want, signals.IDs)
		}
	}
	if _, ok := signals.Routes["/messages/{message_id}"]; !ok {
		t.Fatalf("missing route: %#v", signals.Routes)
	}
	for _, want := range []string{"messages", "delete"} {
		if _, ok := signals.PathParts[want]; !ok {
			t.Fatalf("missing path part %s in %#v", want, signals.PathParts)
		}
	}
	if _, ok := signals.Entities["messages"]; !ok {
		t.Fatalf("missing entity from SQL patch: %#v", signals.Entities)
	}
	if _, ok := signals.Skills["api-contract-review"]; !ok {
		t.Fatalf("missing selected skill: %#v", signals.Skills)
	}
}

func TestBuildEvidenceManifestIncludesAuthorityLevel(t *testing.T) {
	manifest := buildEvidenceManifest([]scoredCorpusSection{
		{
			section: corpusSection{
				Section:   review.Section{Path: "docs/CONTRACT.md", Title: "REQ-12", Kind: review.KindContract, Content: "REQ-12 contract"},
				Authority: "contract",
			},
			score:          900,
			matchedSignals: []string{"REQ-12"},
			reason:         "requirement_trace: REQ-12",
		},
		{
			section: corpusSection{
				Section:   review.Section{Path: "docs/DESIGN.md", Title: "Composer", Kind: review.KindDesign, Content: "Composer states"},
				Authority: "design",
			},
			score:  500,
			reason: "ui_trace: Composer",
		},
	})

	if len(manifest) != 2 {
		t.Fatalf("expected two manifest items, got %#v", manifest)
	}
	if manifest[0].AuthorityLevel != "sot" || !manifest[0].CanJustifyFinding || manifest[0].SupportsOnly {
		t.Fatalf("contract manifest item should be source of truth: %#v", manifest[0])
	}
	if manifest[1].AuthorityLevel != "design_context" || manifest[1].CanJustifyFinding || !manifest[1].SupportsOnly {
		t.Fatalf("design manifest item should be supporting design context: %#v", manifest[1])
	}
}

func TestExtractReviewSignals_IncludesSCMCommitMessages(t *testing.T) {
	req := review.Request{
		Title:        "Cleanup send flow",
		Description:  "Small implementation follow-up.",
		ChangedPaths: []string{"services/backend/app/messaging/send.py"},
	}
	scm := &review.SCMContext{Commits: []review.Commit{{
		SHA:     "abc123",
		Title:   "MSGCORE-09 OPC-SEND resend dedup",
		Message: "Preserve INV-4 and PRO-6 when the retry path reuses /messages and client_idempotency_key.",
	}}}

	signals := extractReviewSignals(req, nil, scm, nil)

	for _, want := range []string{"OPC-SEND", "INV-4", "PRO-6"} {
		if _, ok := signals.IDs[want]; !ok {
			t.Fatalf("missing id %s from commit metadata in %#v", want, signals.IDs)
		}
	}
	if _, ok := signals.Routes["/messages"]; !ok {
		t.Fatalf("missing route from commit metadata: %#v", signals.Routes)
	}
	if _, ok := signals.CodeRules["python"]; !ok {
		t.Fatalf("missing code rule from changed path: %#v", signals.CodeRules)
	}
}

func TestExtractReviewSignals_GenericIdentifierPattern(t *testing.T) {
	req := review.Request{
		Title:       "Trace FR-MSG-42 REQ-12 UC-4 RULE-8 ADR-003",
		Description: "Ignore file-looking uppercase tokens like QUALITY-GATES.MD.",
	}

	signals := extractReviewSignals(req, nil, nil, nil)

	for _, want := range []string{"FR-MSG-42", "REQ-12", "UC-4", "RULE-8", "ADR-003"} {
		if _, ok := signals.IDs[want]; !ok {
			t.Fatalf("missing generic id %s in %#v", want, signals.IDs)
		}
	}
	if _, ok := signals.IDs["QUALITY-GATES.MD"]; ok {
		t.Fatalf("file-like token was treated as corpus id: %#v", signals.IDs)
	}
}

func TestSplitCorpusDocument_MarkdownHeadings(t *testing.T) {
	doc := corpusDocument{
		Path:      "docs/SRS.md",
		Kind:      review.KindPlanning,
		Authority: "requirements",
		Content:   "# Messaging\nFR-1 send messages\n\n## Deletion\nINV-9 tombstones apply",
	}

	sections := splitCorpusDocument(doc)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %#v", sections)
	}
	if sections[0].Title != "Messaging" || sections[1].Title != "Deletion" {
		t.Fatalf("unexpected headings: %#v", sections)
	}
	if !strings.Contains(sections[1].Content, "INV-9") {
		t.Fatalf("section content missing invariant: %q", sections[1].Content)
	}
}

func TestCorpusGraph_ClassifiesIDsByDocumentContext(t *testing.T) {
	sections := []corpusSection{
		{Section: review.Section{Path: "docs/SRS.md", Title: "REQ-12", Content: "REQ-12 login", Kind: review.KindPlanning}, Authority: "requirements"},
		{Section: review.Section{Path: "docs/CONTRACT.md", Title: "REQ-12 constraint", Content: "REQ-12 login invariant", Kind: review.KindContract}, Authority: "contract"},
		{Section: review.Section{Path: "docs/adr/ADR-003.md", Title: "ADR-003", Content: "ADR-003 session storage", Kind: review.KindArchitecture}, Authority: "architecture"},
		{Section: review.Section{Path: "docs/openapi.yaml", Title: "paths./sessions", Content: "operationId: createSession\nREQ-12", Kind: review.KindAPI}, Authority: "api_contract"},
		{Section: review.Section{Path: "docs/DESIGN.md", Title: "REQ-12 login", Content: "REQ-12 login form", Kind: review.KindDesign}, Authority: "design"},
	}

	graph := buildCorpusGraph(sections)

	wants := map[string]string{
		"docs/SRS.md":         "requirement",
		"docs/CONTRACT.md":    "constraint",
		"docs/adr/ADR-003.md": "decision",
		"docs/openapi.yaml":   "interface",
		"docs/DESIGN.md":      "design",
	}
	for _, node := range graph.Nodes {
		for id, wantClass := range wants {
			if node.Section.Path != id {
				continue
			}
			var corpusID string
			if node.Section.Path == "docs/adr/ADR-003.md" {
				corpusID = "ADR-003"
			} else {
				corpusID = "REQ-12"
			}
			if got := node.IDs[corpusID]; got != wantClass {
				t.Fatalf("%s classified %q, want %q", node.Section.Path, got, wantClass)
			}
		}
	}
}

func TestCorpusGraph_BuildsTypedEdges(t *testing.T) {
	sections := []corpusSection{
		{Section: review.Section{Path: "docs/SRS.md", Title: "REQ-12", Content: "REQ-12 UC-4 login", Kind: review.KindPlanning}, Authority: "requirements"},
		{Section: review.Section{Path: "docs/CONTRACT.md", Title: "RULE-8", Content: "REQ-12 RULE-8 UC-4 ADR-003", Kind: review.KindContract}, Authority: "contract"},
		{Section: review.Section{Path: "docs/security.md", Title: "RULE-8", Content: "RULE-8 protects sessions", Kind: review.KindSecurity}, Authority: "security"},
		{Section: review.Section{Path: "docs/adr/ADR-003.md", Title: "ADR-003", Content: "ADR-003 selected session store", Kind: review.KindArchitecture}, Authority: "architecture"},
		{Section: review.Section{Path: "docs/openapi.yaml", Title: "paths./sessions", Content: "$ref: '#/components/schemas/Sessions'\nREQ-12", Kind: review.KindAPI}, Authority: "api_contract"},
		{Section: review.Section{Path: "docs/openapi.yaml", Title: "schemas.Sessions", Content: "type: object", Kind: review.KindAPI}, Authority: "api_contract"},
		{Section: review.Section{Path: "docs/DATA-MODEL.md", Title: "sessions", Content: "table sessions stores login sessions", Kind: review.KindContract}, Authority: "contract"},
		{Section: review.Section{Path: "docs/DESIGN.md", Title: "Composer", Content: "composer layout", Kind: review.KindDesign}, Authority: "design", HeadingPath: []string{"Composer"}, Level: 1, Ordinal: 0},
		{Section: review.Section{Path: "docs/DESIGN.md", Title: "Accessibility", Content: "composer keyboard state", Kind: review.KindDesign}, Authority: "design", HeadingPath: []string{"Composer", "Accessibility"}, Level: 2, Ordinal: 1},
		{Section: review.Section{Path: "docs/OWNERSHIP.md", Title: "Composer ownership", Content: "composer owner", Kind: review.KindArchitecture}, Authority: "architecture"},
	}

	graph := buildCorpusGraph(sections)

	for _, want := range []struct {
		edgeType string
		label    string
	}{
		{"requirement_trace", "REQ-12"},
		{"constraint_trace", "RULE-8"},
		{"operation_trace", "UC-4"},
		{"decision_trace", "ADR-003"},
		{"interface_trace", "sessions"},
		{"data_trace", "sessions"},
		{"ui_trace", "composer"},
		{"ownership_trace", "composer"},
		{"hierarchy", "parent Composer"},
	} {
		if !graphHasEdge(graph, want.edgeType, want.label) {
			t.Fatalf("missing %s edge with label %q", want.edgeType, want.label)
		}
	}
}

func TestExpandGraphEvidence_RespectsDepthAndPerSeedLimit(t *testing.T) {
	sections := []corpusSection{
		{Section: review.Section{Path: "docs/SRS.md", Title: "REQ-12", Content: "REQ-12", Kind: review.KindPlanning}, Authority: "requirements"},
		{Section: review.Section{Path: "docs/CONTRACT.md", Title: "REQ-12 contract", Content: "REQ-12", Kind: review.KindContract}, Authority: "contract"},
		{Section: review.Section{Path: "docs/openapi.yaml", Title: "paths./sessions", Content: "REQ-12", Kind: review.KindAPI}, Authority: "api_contract"},
		{Section: review.Section{Path: "docs/DATA-MODEL.md", Title: "sessions", Content: "REQ-12", Kind: review.KindContract}, Authority: "contract"},
		{Section: review.Section{Path: "docs/DESIGN.md", Title: "Sessions", Content: "REQ-12", Kind: review.KindDesign}, Authority: "design"},
	}
	graph := buildCorpusGraph(sections)
	seed := graphSeed{Node: 0, Kind: "identifier", Value: "REQ-12", Score: 240}
	initial := []scoredCorpusSection{{section: sections[0], score: 240, matchedSignals: []string{"REQ-12"}, reason: "seed: identifier REQ-12"}}

	expanded := expandGraphEvidence(graph, initial, []graphSeed{seed}, graphExpansionLimits{PerSeed: 2})

	if len(expanded) != 3 {
		t.Fatalf("expanded %d sections, want seed + 2 related: %#v", len(expanded), expanded)
	}
	for _, item := range expanded[1:] {
		if item.reason == "" || !strings.Contains(item.reason, "_trace") {
			t.Fatalf("expanded item missing graph trace reason: %#v", item)
		}
	}
}

func TestExpandGraphHierarchyEvidence_ExplainsParentPath(t *testing.T) {
	sections := []corpusSection{
		{
			Section:     review.Section{Path: "docs/DESIGN.md", Title: "Composer", Content: "Composer design contract.", Kind: review.KindDesign},
			Authority:   "design",
			HeadingPath: []string{"Composer"},
			Level:       1,
			Ordinal:     0,
		},
		{
			Section:     review.Section{Path: "docs/DESIGN.md", Title: "Accessibility", Content: "Accessibility requirements.", Kind: review.KindDesign},
			Authority:   "design",
			HeadingPath: []string{"Composer", "Accessibility"},
			Level:       2,
			Ordinal:     1,
		},
		{
			Section:     review.Section{Path: "docs/DESIGN.md", Title: "Keyboard navigation", Content: "Keyboard focus stays visible.", Kind: review.KindDesign},
			Authority:   "design",
			HeadingPath: []string{"Composer", "Accessibility", "Keyboard navigation"},
			Level:       3,
			Ordinal:     2,
		},
	}
	graph := buildCorpusGraph(sections)
	initial := []scoredCorpusSection{{
		section:        sections[2],
		score:          90,
		matchedSignals: []string{"keyboard"},
		reason:         "design: query term keyboard",
	}}

	expanded := expandGraphHierarchyEvidence(graph, initial, graphExpansionLimits{PerSeed: 2})

	assertScoredSectionReason(t, expanded, "docs/DESIGN.md", "Accessibility", "hierarchy: parent Accessibility selected with docs/DESIGN.md#Keyboard navigation")
	assertScoredSectionReason(t, expanded, "docs/DESIGN.md", "Composer", "hierarchy: parent Composer selected with docs/DESIGN.md#Accessibility")
}

func TestSplitCorpusDocument_OpenAPIYAMLSections(t *testing.T) {
	doc := corpusDocument{
		Path:      "docs/openapi.yaml",
		Kind:      review.KindAPI,
		Authority: "api_contract",
		Content: `openapi: 3.0.0
paths:
  /messages/{message_id}:
    delete:
      operationId: deleteMessage
  /threads:
    get:
      operationId: listThreads
components:
  schemas:
    Message:
      type: object
`,
	}

	sections := splitCorpusDocument(doc)

	got := sectionTitles(sections)
	for _, want := range []string{"paths./messages/{message_id}", "paths./threads", "schemas.Message"} {
		if !containsString(got, want) {
			t.Fatalf("missing %s in %#v", want, got)
		}
	}
}

func TestSplitCorpusDocument_AsyncAPIJSONSections(t *testing.T) {
	doc := corpusDocument{
		Path:      "docs/asyncapi.json",
		Kind:      review.KindAPI,
		Authority: "api_contract",
		Content:   `{"asyncapi":"2.6.0","channels":{"messages/deleted":{"publish":{"message":{"$ref":"#/components/messages/Deleted"}}}},"components":{"messages":{"Deleted":{"payload":{"type":"object"}}}}}`,
	}

	sections := splitCorpusDocument(doc)

	got := sectionTitles(sections)
	for _, want := range []string{"channels.messages/deleted", "messages.Deleted"} {
		if !containsString(got, want) {
			t.Fatalf("missing %s in %#v", want, got)
		}
	}
}

func TestSelectCorpus_MessengerBackendDeleteForAllEvidence(t *testing.T) {
	root := messengerRepoRoot(t)
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "aiobi-messenger",
		ChangeID:    "7",
		Title:       "Delete all messages OPC-DELALL",
		Description: "Fix INV-9, LAW-1, and /messages/{message_id}",
	})
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{
		Path: "services/backend/app/messages.py",
		Patch: `+@router.delete("/messages/{message_id}")
+async def delete_message(message_id: str, scope: str = "all"):
+    await conn.execute("UPDATE message_envelopes SET deleted_for_all = true WHERE id = $1", message_id)
+    await conn.execute("UPDATE outbox_events SET payload = payload - 'ciphertext' WHERE payload->>'message_id' = $1", message_id)
+    await conn.execute("UPDATE message_edits SET body_snapshot = NULL WHERE message_id = $1", message_id)
+    await redis.delete(f"msg:snapshot:{message_id}")`,
	}}}
	rc.Request.ChangedPaths = rc.ChangedPaths()
	rc.SkillSections = []review.Section{{Title: "api-contract-review"}, {Title: "data-migration-review"}}

	sections, manifest, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}

	assertSelectedContent(t, sections, []string{"OPC-DELALL", "INV-9", "LAW-1", "/messages/{message_id}", "PY-1", "deleted_for_all", "message_deletions", "outbox_events", "message_edits"})
	assertSupportingSectionCount(t, sections, defaultMaxSupportingCorpusSections)
	assertSelectedSources(t, sections, []string{
		"planning-and-design-sdlc-1/Planning/03-CONTRACT.md",
		"planning-and-design-sdlc-1/Design/openapi.yaml",
		"planning-and-design-sdlc-1/Design/06-DATA-MODEL.md",
		"RULES/code/python.md",
	})
	assertNotSelectedContent(t, sections, []string{"ORG-B2", "Composer un rôle admin", "revokeAdminRole"})
	assertNotSelectedSources(t, sections, []string{"planning-and-design-sdlc-1/Design/adr/ADR-0004-isolation-livekit.md"})
	assertNoWholeOpenAPILeak(t, sections, "/admin/orgs")
	if len(sections) != len(manifest) {
		t.Fatalf("manifest length %d does not match selected sections %d", len(manifest), len(sections))
	}
	if len(manifest) == 0 {
		t.Fatal("expected evidence manifest")
	}
	if manifest[0].SelectionReason == "" || manifest[0].Score == 0 || manifest[0].Source == "" {
		t.Fatalf("manifest missing explanation fields: %#v", manifest[0])
	}
	assertManifestMatchesSections(t, manifest, sections)
	assertManifestContainsReason(t, manifest, "planning-and-design-sdlc-1/Planning/03-CONTRACT.md", "INV-9", "identifier INV-9")
	assertManifestContainsReason(t, manifest, "planning-and-design-sdlc-1/Planning/03-CONTRACT.md", "LAW-1", "identifier LAW-1")
	assertManifestContainsReason(t, manifest, "planning-and-design-sdlc-1/Design/openapi.yaml", "paths./messages/{message_id}", "API route /messages/{message_id}")
}

func TestSelectCorpus_MessengerSparseBackendSendChangeUsesRealPathSignals(t *testing.T) {
	root := messengerRepoRoot(t)
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "aiobi-messenger",
		ChangeID:    "9",
		Title:       "Fix resend deduplication",
		Description: "Implementation cleanup.",
	})
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{
		Path: "services/backend/app/messaging/send.py",
		Patch: `@@
-    return deduplicate(store, key, mint=new_message_id)
+    message_id, was_resend = deduplicate(store, key, mint=new_message_id)
+    return message_id, was_resend`,
	}}}
	rc.Request.ChangedPaths = rc.ChangedPaths()
	rc.Source.SCM = &review.SCMContext{Commits: []review.Commit{{
		SHA:     "abc123",
		Title:   "MSGCORE-09 OPC-SEND resend dedup",
		Message: "Preserve INV-4 and PRO-6 when retrying send with client_idempotency_key.",
	}}}
	rc.SkillSections = []review.Section{{Title: "api-contract-review"}, {Title: "data-migration-review"}, {Title: "traceability-review"}}
	signals := extractReviewSignals(rc.Request, rc.Diff, rc.Source.SCM, rc.SkillSections)
	if _, ok := signals.CodeRules["python"]; !ok {
		t.Fatalf("fixture did not produce python code-rule signal: %#v", signals.CodeRules)
	}

	sections, manifest, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}

	assertSelectedContent(t, sections, []string{"OPC-SEND", "INV-4", "PRO-6", "message_envelopes", "client_idempotency_key"})
	assertSelectedSources(t, sections, []string{
		"planning-and-design-sdlc-1/Planning/03-CONTRACT.md",
		"planning-and-design-sdlc-1/Design/06-DATA-MODEL.md",
		"RULES/code/python.md",
	})
	assertNotSelectedSources(t, sections, []string{"RULES/code/go.md", "RULES/code/typescript.md"})
	assertNotSelectedSectionRefs(t, sections, []string{
		"planning-and-design-sdlc-1/Design/openapi.yaml#paths./messages/{message_id}",
		"planning-and-design-sdlc-1/Design/adr/ADR-0004-isolation-livekit.md",
	})
	assertNotSelectedContent(t, sections, []string{"Composer un rôle admin", "revokeAdminRole", "LiveKit"})
	assertManifestContainsReason(t, manifest, "planning-and-design-sdlc-1/Planning/03-CONTRACT.md", "OPC-SEND — Émission d'un message", "identifier OPC-SEND")
	assertManifestContainsReason(t, manifest, "RULES/code/python.md", "Lois Python — services/backend", "changed code language python")
}

func TestSelectCorpus_GenericGitHubRepoUsesPathsDiffAndCommits(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "AGENTS.md", "# Repo rules\nKeep reviews contract-backed.")
	writeCorpusFile(t, root, "RULES/code/python.md", "# PY-1\nPython handlers keep auth policy in services.")
	writeCorpusFile(t, root, "RULES/code/go.md", "# GO-1\nGo handlers use context deadlines.")
	writeCorpusFile(t, root, "docs/CONTRACT.md", "# OPC-LOGIN\nLogin creates a server session.\n\n## INV-AUTH\nA successful login must rotate the session token.")
	writeCorpusFile(t, root, "docs/openapi.yaml", `openapi: 3.0.0
paths:
  /sessions:
    post:
      operationId: createSession
  /admin/reports:
    get:
      operationId: listReports
`)
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "generic-web-app",
		ChangeID:    "42",
		Title:       "small auth cleanup",
		Description: "Follow-up from review.",
		Labels:      []string{"security"},
	})
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{
		Path: "src/accounts/views.py",
		Patch: `@@
+session.rotate_token(user_id)
+audit.write("login", user_id)`,
	}}}
	rc.Request.ChangedPaths = rc.ChangedPaths()
	rc.Source.SCM = &review.SCMContext{Commits: []review.Commit{{
		SHA:     "def456",
		Title:   "OPC-LOGIN session rotation",
		Message: "Preserve INV-AUTH while updating POST /sessions.",
	}}}
	rc.SkillSections = []review.Section{{Title: "api-contract-review"}, {Title: "security-review"}}

	sections, manifest, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}

	assertSelectedContent(t, sections, []string{"OPC-LOGIN", "INV-AUTH", "/sessions", "PY-1"})
	assertSelectedSources(t, sections, []string{"docs/CONTRACT.md", "docs/openapi.yaml", "RULES/code/python.md"})
	assertNotSelectedSources(t, sections, []string{"RULES/code/go.md"})
	assertNoWholeOpenAPILeak(t, sections, "/admin/reports")
	assertManifestContainsReason(t, manifest, "docs/CONTRACT.md", "OPC-LOGIN", "identifier OPC-LOGIN")
	assertManifestContainsReason(t, manifest, "docs/openapi.yaml", "paths./sessions", "API route /sessions")
	assertManifestContainsReason(t, manifest, "RULES/code/python.md", "PY-1", "baseline code-zone rules")
}

func TestSelectCorpus_ManifestExplainsGraphTracePaths(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "docs/openapi.yaml", `openapi: 3.0.0
paths:
  /sessions:
    post:
      operationId: createSession
      responses:
        "201":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Session"
components:
  schemas:
    Session:
      type: object
      properties:
        id:
          type: string
`)
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "generic-api",
		ChangeID:    "51",
		Title:       "Update session creation",
		Description: "POST /sessions",
	})

	sections, manifest, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}

	assertSelectedContent(t, sections, []string{"/sessions", "Session"})
	assertManifestContainsReason(t, manifest, "docs/openapi.yaml", "paths./sessions", "API route /sessions")
	assertManifestContainsReason(t, manifest, "docs/openapi.yaml", "schemas.Session", "interface_trace: session -> docs/openapi.yaml#schemas.Session")
}

func TestSelectCorpus_MessengerWebAndGatewayRouting(t *testing.T) {
	root := messengerRepoRoot(t)

	web := review.NewContext(review.Request{Provider: "github", ProjectID: "aiobi-messenger", ChangeID: "1", Title: "Composer accessibility loading state"})
	web.Diff = &review.StructuredDiff{Files: []review.FileDiff{{Path: "clients/web/src/components/Composer.tsx", Patch: "+<footer id=\"composer\"><button disabled={loading}>Send</button></footer>"}}}
	web.Request.ChangedPaths = web.ChangedPaths()
	web.SkillSections = []review.Section{{Title: "design-contract-review"}, {Title: "frontend-accessibility-review"}}

	webSections, webManifest, err := selectCorpus(context.Background(), root, web, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}
	assertSelectedContent(t, webSections, []string{"Lois TypeScript", "composer", "Accessibility", "Navigation clavier"})
	assertSupportingSectionCount(t, webSections, defaultMaxSupportingCorpusSections)
	assertSelectedSources(t, webSections, []string{"RULES/code/typescript.md", "DESIGN.md"})
	assertManifestContainsReason(t, webManifest, "DESIGN.md", "Accessibility (WCAG 2.2 AA)", "query term accessibility")

	gateway := review.NewContext(review.Request{Provider: "github", ProjectID: "aiobi-messenger", ChangeID: "2", Title: "GAR-1 websocket Redis fanout"})
	gateway.Diff = &review.StructuredDiff{Files: []review.FileDiff{{Path: "services/gateway/internal/hub/hub.go", Patch: "+redis.Publish(ctx, channel, outboxEvent)\n+conn.Write(writeCtx, websocket.MessageText, payload)"}}}
	gateway.Request.ChangedPaths = gateway.ChangedPaths()
	gateway.SkillSections = []review.Section{{Title: "reliability-review"}}

	gatewaySections, _, err := selectCorpus(context.Background(), root, gateway, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}
	assertSelectedContent(t, gatewaySections, []string{"LOI-G2", "GAR-1", "Redis", "WebSocket"})
	assertSupportingSectionCount(t, gatewaySections, defaultMaxSupportingCorpusSections)
	assertSelectedSources(t, gatewaySections, []string{"RULES/code/go.md", "planning-and-design-sdlc-1/Planning/03-CONTRACT.md"})
	assertNotSelectedContent(t, gatewaySections, []string{"composer-reply-preview", "Suppression « pour tous »", "OPC-DELALL"})
}

func TestSelectCorpus_SupportingSectionCapIsConfigurable(t *testing.T) {
	root := messengerRepoRoot(t)
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "aiobi-messenger",
		ChangeID:    "8",
		Title:       "GAR-1 websocket Redis fanout architecture",
		Description: "Review GAR-1, CMP-RT, ADR-0001 and Redis WebSocket fanout.",
	})
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{
		Path:  "services/gateway/internal/hub/hub.go",
		Patch: "+redis.Publish(ctx, channel, payload)\n+conn.Write(writeCtx, websocket.MessageText, payload)",
	}}}
	rc.Request.ChangedPaths = rc.ChangedPaths()
	rc.SkillSections = []review.Section{{Title: "reliability-review"}, {Title: "traceability-review"}}

	defaultSections, _, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}
	expandedSections, _, err := selectCorpus(context.Background(), root, rc, 5)
	if err != nil {
		t.Fatal(err)
	}

	if got := supportingSectionCount(defaultSections); got > defaultMaxSupportingCorpusSections {
		t.Fatalf("default cap selected %d supporting sections, want <= %d", got, defaultMaxSupportingCorpusSections)
	}
	if got := supportingSectionCount(expandedSections); got <= defaultMaxSupportingCorpusSections {
		t.Fatalf("expanded cap did not allow more supporting sections: got %d, default cap %d; selected=%v", got, defaultMaxSupportingCorpusSections, sectionRefs(expandedSections))
	}
	t.Logf("default cap=%d supporting=%v", defaultMaxSupportingCorpusSections, supportingSectionRefs(defaultSections))
	t.Logf("expanded cap=%d supporting=%v", 5, supportingSectionRefs(expandedSections))
	assertSupportingSectionCount(t, expandedSections, 5)
}

func TestSelectCorpus_DocsMechanicalChangeStaysSmall(t *testing.T) {
	root := t.TempDir()
	writeCorpusFile(t, root, "AGENTS.md", "Repository rules.")
	writeCorpusFile(t, root, "docs/security.md", "# Auth\nLAW-1 protects secrets.")
	writeCorpusFile(t, root, "docs/openapi.yaml", "openapi: 3.0.0\npaths:\n  /admin/secrets:\n    get: {}\n")
	rc := review.NewContext(review.Request{
		Provider:    "github",
		ProjectID:   "m",
		ChangeID:    "3",
		Title:       "Fix README typo",
		Description: "No product behavior change.",
	})
	rc.Diff = &review.StructuredDiff{Files: []review.FileDiff{{Path: "README.md", Patch: "+typo"}}}
	rc.Request.ChangedPaths = rc.ChangedPaths()

	sections, _, err := selectCorpus(context.Background(), root, rc, defaultMaxSupportingCorpusSections)
	if err != nil {
		t.Fatal(err)
	}

	if len(sections) > 2 {
		t.Fatalf("mechanical docs change selected too much context: %#v", sections)
	}
	for _, section := range sections {
		if strings.Contains(section.Content, "/admin/secrets") || strings.Contains(section.Content, "LAW-1") {
			t.Fatalf("selected unrelated security/API evidence: %#v", sections)
		}
	}
}

func writeCorpusFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func messengerRepoRoot(t *testing.T) string {
	t.Helper()
	if root := strings.TrimSpace(os.Getenv("MESSENGER_REPO")); root != "" {
		if isDir(root) {
			return root
		}
		t.Skipf("MESSENGER_REPO does not exist: %s", root)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	candidates := []string{
		filepath.Join(home, "Projects", "Aïobi-Messenger", "aiobi-messenger"),
		filepath.Join(home, "Projects", "Aiobi-Messenger", "aiobi-messenger"),
	}
	for _, candidate := range candidates {
		if isDir(candidate) {
			return candidate
		}
	}
	t.Skip("Messenger repository not found; set MESSENGER_REPO to run integration corpus-selection tests")
	return ""
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func sectionTitles(sections []corpusSection) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		out = append(out, section.Title)
	}
	return out
}

func selectedTitlesAndContent(sections []review.Section) string {
	var b strings.Builder
	for _, section := range sections {
		b.WriteString(section.Title)
		b.WriteByte('\n')
		b.WriteString(section.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

func assertSelectedContent(t *testing.T, sections []review.Section, wants []string) {
	t.Helper()
	joined := selectedTitlesAndContent(sections)
	for _, want := range wants {
		if !strings.Contains(joined, want) {
			t.Fatalf("selected corpus missing %q; selected=%v", want, sectionRefs(sections))
		}
	}
}

func assertSelectedSources(t *testing.T, sections []review.Section, wants []string) {
	t.Helper()
	sources := make(map[string]struct{})
	for _, section := range sections {
		sources[section.Path] = struct{}{}
	}
	for _, want := range wants {
		if _, ok := sources[want]; !ok {
			t.Fatalf("selected corpus missing source %q; got %v", want, sortedSectionSources(sections))
		}
	}
}

func assertNotSelectedSources(t *testing.T, sections []review.Section, forbidden []string) {
	t.Helper()
	sources := make(map[string]struct{})
	for _, section := range sections {
		sources[section.Path] = struct{}{}
	}
	for _, value := range forbidden {
		if _, ok := sources[value]; ok {
			t.Fatalf("selected unrelated source %q; selected=%v", value, sortedSectionSources(sections))
		}
	}
}

func assertNotSelectedSectionRefs(t *testing.T, sections []review.Section, forbidden []string) {
	t.Helper()
	refs := make(map[string]struct{})
	for _, section := range sections {
		refs[section.Path+"#"+section.Title] = struct{}{}
	}
	for _, value := range forbidden {
		if _, ok := refs[value]; ok {
			t.Fatalf("selected unrelated section %q; selected=%v", value, sectionRefs(sections))
		}
	}
}

func assertSupportingSectionCount(t *testing.T, sections []review.Section, limit int) {
	t.Helper()
	count := supportingSectionCount(sections)
	if count > limit {
		t.Fatalf("selected %d supporting system/traceability sections, want <= %d; selected=%v", count, limit, sectionRefs(sections))
	}
}

func supportingSectionCount(sections []review.Section) int {
	count := 0
	for _, section := range sections {
		if isSupportingCorpusSection(corpusSection{Section: section}) {
			count++
		}
	}
	return count
}

func supportingSectionRefs(sections []review.Section) []string {
	var out []string
	for _, section := range sections {
		if isSupportingCorpusSection(corpusSection{Section: section}) {
			out = append(out, section.Path+"#"+section.Title)
		}
	}
	sort.Strings(out)
	return out
}

func sortedSectionSources(sections []review.Section) []string {
	sources := make(map[string]struct{})
	for _, section := range sections {
		sources[section.Path] = struct{}{}
	}
	out := make([]string, 0, len(sources))
	for source := range sources {
		out = append(out, source)
	}
	sort.Strings(out)
	return out
}

func assertNotSelectedContent(t *testing.T, sections []review.Section, forbidden []string) {
	t.Helper()
	joined := selectedTitlesAndContent(sections)
	for _, value := range forbidden {
		if strings.Contains(joined, value) {
			t.Fatalf("selected corpus included unrelated %q; selected=%v", value, sectionRefs(sections))
		}
	}
}

func sectionRefs(sections []review.Section) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		out = append(out, section.Path+"#"+section.Title)
	}
	sort.Strings(out)
	return out
}

func assertNoWholeOpenAPILeak(t *testing.T, sections []review.Section, forbidden string) {
	t.Helper()
	for _, section := range sections {
		if section.Kind == review.KindAPI && strings.Contains(section.Content, forbidden) {
			t.Fatalf("API section %s/%s leaked unrelated path %q:\n%s", section.Path, section.Title, forbidden, section.Content)
		}
	}
}

func assertManifestMatchesSections(t *testing.T, manifest []review.EvidenceItem, sections []review.Section) {
	t.Helper()
	for i := range sections {
		if manifest[i].Source != sections[i].Path || manifest[i].HeadingOrKey != sections[i].Title || manifest[i].Kind != sections[i].Kind {
			t.Fatalf("manifest[%d] does not describe section[%d]: manifest=%#v section=%#v", i, i, manifest[i], sections[i])
		}
		if manifest[i].ContentBytes != len(sections[i].Content) {
			t.Fatalf("manifest[%d] content bytes=%d, want %d", i, manifest[i].ContentBytes, len(sections[i].Content))
		}
	}
}

func assertManifestContainsReason(t *testing.T, manifest []review.EvidenceItem, source, heading, reason string) {
	t.Helper()
	for _, item := range manifest {
		if item.Source == source && item.HeadingOrKey == heading {
			if !strings.Contains(item.SelectionReason, reason) {
				t.Fatalf("manifest item %s/%s reason %q does not contain %q", source, heading, item.SelectionReason, reason)
			}
			return
		}
	}
	t.Fatalf("manifest missing item %s/%s in %#v", source, heading, manifest)
}

func assertScoredSectionReason(t *testing.T, scored []scoredCorpusSection, source, heading, reason string) {
	t.Helper()
	for _, item := range scored {
		if item.section.Path == source && item.section.Title == heading {
			if !strings.Contains(item.reason, reason) {
				t.Fatalf("scored item %s/%s reason %q does not contain %q", source, heading, item.reason, reason)
			}
			return
		}
	}
	t.Fatalf("scored items missing %s/%s in %#v", source, heading, scored)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func graphHasEdge(graph CorpusGraph, edgeType, label string) bool {
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			if edge.Type == edgeType && edge.Label == label {
				return true
			}
		}
	}
	return false
}
