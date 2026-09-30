package jev_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/haha-systems/qac/jev"
)

const mixedResponse = `{"model":"jev-1.13.0","answers":{"resource":{"type":"choice","choice":"large","confidence":0.8,"probabilities":{"small":0.1,"large":0.9}},"difficulty":{"type":"score","score":1.7,"confidence":0.6,"legend":{"0":"easy","1":{"label":"moderate"},"2":["hard"]},"probabilities":{"0":0.1,"1":0.1,"2":0.8}},"ready":{"type":"noul","noul":0}},"usage":{"input_tokens":120,"output_tokens":0},"future_field":true}`

func TestSystemOneMixedQuestions(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
		}

		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing authentication or JSON header")
		}

		var got any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}

		var want any
		_ = json.Unmarshal([]byte(`{"model":"jev-latest","state":{"task":"repair","attempts":2},"questions":{"resource":{"type":"choice","instructions":{"question":"Which resource?"},"criteria":{"small":null,"large":{"capability":"deep reasoning"}}},"difficulty":{"type":"score","instructions":["Rate difficulty"],"criteria":["easy",{"label":"moderate"},["hard"]]},"ready":{"type":"noul","instructions":"Ready?","criteria":{"true":"ready","false":["blocked"]}}}}`), &want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request = %#v, want %#v", got, want)
		}

		_, _ = w.Write([]byte(mixedResponse))
	})

	result, err := client.SystemOne(context.Background(), jev.Request{
		State: map[string]any{"task": "repair", "attempts": 2},
		Questions: map[string]jev.Question{
			"resource":   jev.ChoiceQuestion{Instructions: map[string]any{"question": "Which resource?"}, Criteria: map[string]any{"small": nil, "large": map[string]any{"capability": "deep reasoning"}}},
			"difficulty": jev.ScoreQuestion{Instructions: []string{"Rate difficulty"}, Criteria: []any{"easy", map[string]any{"label": "moderate"}, []string{"hard"}}},
			"ready":      jev.NoulQuestion{Instructions: "Ready?", Criteria: &jev.NoulCriteria{True: "ready", False: []string{"blocked"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	choice, ok := result.Answers["resource"].(jev.ChoiceAnswer)
	if !ok || choice.Choice != "large" || choice.Confidence != .8 || choice.Probabilities["large"] != .9 {
		t.Fatalf("choice = %#v", result.Answers["resource"])
	}

	score, ok := result.Answers["difficulty"].(jev.ScoreAnswer)
	if !ok || score.Score != 1.7 || score.Confidence != .6 || score.Probabilities["2"] != .8 || !reflect.DeepEqual(score.Legend["1"], map[string]any{"label": "moderate"}) {
		t.Fatalf("score = %#v", result.Answers["difficulty"])
	}

	noul, ok := result.Answers["ready"].(jev.NoulAnswer)
	if !ok || noul.Noul != 0 || result.Model != "jev-1.13.0" || result.Usage.InputTokens != 120 || result.Usage.OutputTokens != 0 {
		t.Fatalf("response = %#v", result)
	}
}

func TestModelOverrideAndArrayState(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string   `json:"model"`
			State []string `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}

		if request.Model != "jev-pinned" || !reflect.DeepEqual(request.State, []string{"task", "evidence"}) {
			t.Errorf("request = %#v", request)
		}

		_, _ = w.Write([]byte(noulResponse))
	})

	_, err := client.SystemOne(context.Background(), jev.Request{
		Model: "jev-pinned", State: []string{"task", "evidence"},
		Questions: map[string]jev.Question{"ready": &jev.NoulQuestion{}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListModels(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected models request: %s %s", r.Method, r.URL.Path)
		}

		_, _ = w.Write([]byte(`{"models":[{"name":"jev-latest","description":"Decision model","release_date":"2026-09-15","extra":true}]}`))
	})

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []jev.Model{{Name: "jev-latest", Description: "Decision model", ReleaseDate: "2026-09-15"}}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models = %#v, want %#v", models, want)
	}
}

const noulResponse = `{"model":"jev-1.13.0","answers":{"ready":{"type":"noul","noul":0}},"usage":{"input_tokens":0,"output_tokens":0}}`

func testClient(t *testing.T, handler http.HandlerFunc) *jev.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := jev.New(jev.Config{APIKey: "test-key", BaseURL: server.URL, Retry: &jev.RetryPolicy{MaxRetries: 0}})
	if err != nil {
		t.Fatal(err)
	}

	return client
}
