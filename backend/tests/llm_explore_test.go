package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/portico/backend/internal/connectors"
	"github.com/portico/backend/internal/llm"
	"github.com/portico/backend/internal/models"
	"gorm.io/datatypes"
)

type stubCompleter struct {
	content string
	err     error
}

func (s *stubCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	return s.content, s.err
}

func TestSuggestExploreParsesAndSanitizes(t *testing.T) {
	stub := &stubCompleter{
		content: "```json\n" + `{
			"filters":[
				{"field":"STATUS","operator":"eq","value":"active"},
				{"field":"missing","operator":"eq","value":"x"},
				{"field":"name","operator":"like","value":"%ann%"},
				{"field":"id","operator":"bogus","value":"1"},
				{"field":"email","operator":"is_null","value":"ignored"}
			],
			"fields":["Name","email","nope"]
		}` + "\n```",
	}
	got, err := llm.SuggestExplore(context.Background(), stub, llm.ExploreSuggestInput{
		Prompt:       "active users named ann; show name and email",
		FilterFields: []string{"id", "name", "status", "email"},
		Fields:       []string{"id", "name", "email", "status"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Filters) != 3 {
		t.Fatalf("filters=%+v", got.Filters)
	}
	if got.Filters[0].Field != "status" || got.Filters[0].Operator != "eq" || got.Filters[0].Value != "active" {
		t.Fatalf("filter0=%+v", got.Filters[0])
	}
	if got.Filters[1].Field != "name" || got.Filters[1].Operator != "like" {
		t.Fatalf("filter1=%+v", got.Filters[1])
	}
	if got.Filters[2].Field != "email" || got.Filters[2].Operator != "is_null" || got.Filters[2].Value != "" {
		t.Fatalf("filter2=%+v", got.Filters[2])
	}
	if len(got.Fields) != 2 || got.Fields[0] != "name" || got.Fields[1] != "email" {
		t.Fatalf("fields=%v", got.Fields)
	}
}

func TestSuggestExploreNullFieldsMeansAll(t *testing.T) {
	stub := &stubCompleter{content: `{"filters":[],"fields":null}`}
	got, err := llm.SuggestExplore(context.Background(), stub, llm.ExploreSuggestInput{
		Prompt:       "clear filters",
		FilterFields: []string{"id"},
		Fields:       []string{"id", "name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields != nil {
		t.Fatalf("expected nil fields, got %v", got.Fields)
	}
	if got.Filters == nil || len(got.Filters) != 0 {
		t.Fatalf("expected empty filters, got %#v", got.Filters)
	}
}

func TestSuggestExploreAllFieldsNormalizedToNil(t *testing.T) {
	stub := &stubCompleter{content: `{"filters":[],"fields":["id","name"]}`}
	got, err := llm.SuggestExplore(context.Background(), stub, llm.ExploreSuggestInput{
		Prompt: "all columns",
		Fields: []string{"id", "name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields != nil {
		t.Fatalf("expected nil when all fields selected, got %v", got.Fields)
	}
}

func TestExploreSuggestEndpoint(t *testing.T) {
	gdb := setupTestDB(t)
	srcConn := models.Connection{Name: "src", Type: "mock_src", Config: datatypes.JSON([]byte(`{}`))}
	dstConn := models.Connection{Name: "dst", Type: "mock_dst", Config: datatypes.JSON([]byte(`{}`))}
	if err := gdb.Create(&srcConn).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&dstConn).Error; err != nil {
		t.Fatal(err)
	}
	job := models.SyncJob{
		Name:                    "ai-explore",
		SourceConnectionID:      srcConn.ID,
		DestinationConnectionID: dstConn.ID,
		SourceTable:             "users",
		DestinationTable:        "users",
		ChunkSize:               50,
		Workers:                 1,
	}
	if err := gdb.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/sync-jobs/%d/explore/suggest", job.ID)
	body, _ := json.Marshal(map[string]any{
		"prompt":        "status is active",
		"filter_fields": []string{"status"},
		"fields":        []string{"id", "status"},
	})

	// Not configured → 503
	rNil := setupRouter(t, gdb, connectors.NewRegistry())
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rNil.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("nil llm: status=%d body=%s", w.Code, w.Body.String())
	}

	stub := &stubCompleter{
		content: `{"filters":[{"field":"status","operator":"eq","value":"active"}],"fields":["id","status"]}`,
	}
	r := setupRouterWithLLM(t, gdb, connectors.NewRegistry(), stub)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("suggest: status=%d body=%s", w.Code, w.Body.String())
	}
	var got llm.ExploreSuggestResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Filters) != 1 || got.Filters[0].Field != "status" {
		t.Fatalf("got=%+v", got)
	}
	if got.Fields != nil {
		t.Fatalf("expected null fields, got %v", got.Fields)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/sync-jobs/99999/explore/suggest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing job: status=%d", w.Code)
	}

	empty, _ := json.Marshal(map[string]any{"prompt": "  "})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(empty))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt: status=%d body=%s", w.Code, w.Body.String())
	}
}
