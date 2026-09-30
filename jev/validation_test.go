package jev_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/haha-systems/qac/jev"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	cases := []jev.Config{
		{}, {APIKey: " "}, {APIKey: "secret\r\nvalue"},
		{APIKey: "key", BaseURL: "://bad"},
		{APIKey: "key", BaseURL: "ftp://example.com"},
		{APIKey: "key", BaseURL: "https://user:secret@example.com"},
		{APIKey: "key", BaseURL: "https://example.com?secret=key"},
		{APIKey: "key", BaseURL: "https://example.com#fragment"},
		{APIKey: "key", Timeout: -time.Second},
		{APIKey: "key", Retry: &jev.RetryPolicy{MaxRetries: -1}},
	}
	for i, config := range cases {
		if _, err := jev.New(config); err == nil {
			t.Errorf("case %d: expected configuration error", i)
		} else if strings.Contains(err.Error(), "secret") {
			t.Errorf("case %d: error reveals configuration values", i)
		}
	}
}

func TestInvalidRequestsDoNotSend(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(noulResponse))
	})

	cases := []struct {
		name      string
		state     any
		questions map[string]jev.Question
	}{
		{"missing state", nil, readyQuestions()},
		{"number state", 7, readyQuestions()},
		{"boolean state", false, readyQuestions()},
		{"unsupported state", make(chan int), readyQuestions()},
		{"no questions", "state", nil},
		{"nil question", "state", map[string]jev.Question{"ready": nil}},
		{"nil question pointer", "state", map[string]jev.Question{"ready": (*jev.NoulQuestion)(nil)}},
		{"invalid instructions", "state", map[string]jev.Question{"ready": jev.NoulQuestion{Instructions: false}}},
		{"invalid noul criteria", "state", map[string]jev.Question{"ready": jev.NoulQuestion{Criteria: &jev.NoulCriteria{True: 1}}}},
		{"missing choice criteria", "state", map[string]jev.Question{"ready": jev.ChoiceQuestion{}}},
		{"invalid choice description", "state", map[string]jev.Question{"ready": jev.ChoiceQuestion{Criteria: map[string]any{"a": true}}}},
		{"missing score criteria", "state", map[string]jev.Question{"ready": jev.ScoreQuestion{}}},
		{"empty score criteria", "state", map[string]jev.Question{"ready": jev.ScoreQuestion{Criteria: []any{}}}},
		{"null score level", "state", map[string]jev.Question{"ready": jev.ScoreQuestion{Criteria: []any{nil}}}},
		{"invalid score level", "state", map[string]jev.Question{"ready": jev.ScoreQuestion{Criteria: []any{4}}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.SystemOne(context.Background(), jev.Request{State: test.state, Questions: test.questions})
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	if calls.Load() != 0 {
		t.Fatalf("invalid requests made %d network calls", calls.Load())
	}
}

func TestResponseRequiredFieldsAndAnswerNames(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing model", func(v map[string]any) { delete(v, "model") }},
		{"empty model", func(v map[string]any) { v["model"] = "" }},
		{"missing usage", func(v map[string]any) { delete(v, "usage") }},
		{"missing input tokens", func(v map[string]any) { delete(v["usage"].(map[string]any), "input_tokens") }},
		{"null output tokens", func(v map[string]any) { v["usage"].(map[string]any)["output_tokens"] = nil }},
		{"negative tokens", func(v map[string]any) { v["usage"].(map[string]any)["input_tokens"] = -1 }},
		{"missing answers", func(v map[string]any) { delete(v, "answers") }},
		{"empty answers", func(v map[string]any) { v["answers"] = map[string]any{} }},
		{"wrong answer name", func(v map[string]any) {
			v["answers"] = map[string]any{"other": map[string]any{"type": "noul", "noul": 0}}
		}},
		{"extra answer", func(v map[string]any) {
			v["answers"].(map[string]any)["other"] = map[string]any{"type": "noul", "noul": 0}
		}},
		{"missing noul", func(v map[string]any) { delete(readyAnswer(v), "noul") }},
		{"null noul", func(v map[string]any) { readyAnswer(v)["noul"] = nil }},
		{"noul above one", func(v map[string]any) { readyAnswer(v)["noul"] = 1.1 }},
		{"noul below zero", func(v map[string]any) { readyAnswer(v)["noul"] = -.1 }},
		{"unknown type", func(v map[string]any) { readyAnswer(v)["type"] = "future" }},
		{"mismatched type", func(v map[string]any) {
			v["answers"].(map[string]any)["ready"] = map[string]any{"type": "choice", "choice": "a", "confidence": 1, "probabilities": map[string]any{"a": 1}}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var response map[string]any
			_ = json.Unmarshal([]byte(noulResponse), &response)
			test.change(response)
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			assertBadResponse(t, string(data), readyQuestions())
		})
	}
}

func TestChoiceAndScoreResponseValidation(t *testing.T) {
	choice := map[string]jev.Question{"ready": jev.ChoiceQuestion{Criteria: map[string]any{"a": nil, "b": "other"}}}
	score := map[string]jev.Question{"ready": jev.ScoreQuestion{Criteria: []any{"low", "high"}}}
	cases := []struct {
		name      string
		answer    string
		questions map[string]jev.Question
	}{
		{"unknown choice", `{"type":"choice","choice":"c","confidence":1,"probabilities":{"a":1,"b":0}}`, choice},
		{"missing confidence", `{"type":"choice","choice":"a","probabilities":{"a":1,"b":0}}`, choice},
		{"invalid confidence", `{"type":"choice","choice":"a","confidence":1.1,"probabilities":{"a":1,"b":0}}`, choice},
		{"null probability", `{"type":"choice","choice":"a","confidence":1,"probabilities":{"a":null,"b":1}}`, choice},
		{"negative probability", `{"type":"choice","choice":"a","confidence":1,"probabilities":{"a":-0.1,"b":1.1}}`, choice},
		{"missing probability", `{"type":"choice","choice":"a","confidence":1,"probabilities":{"a":1}}`, choice},
		{"unknown probability", `{"type":"choice","choice":"a","confidence":1,"probabilities":{"a":1,"c":0}}`, choice},
		{"missing score", `{"type":"score","confidence":1,"legend":{"0":"low","1":"high"},"probabilities":{"0":1,"1":0}}`, score},
		{"score above range", `{"type":"score","score":2,"confidence":1,"legend":{"0":"low","1":"high"},"probabilities":{"0":1,"1":0}}`, score},
		{"score below range", `{"type":"score","score":-1,"confidence":1,"legend":{"0":"low","1":"high"},"probabilities":{"0":1,"1":0}}`, score},
		{"missing legend", `{"type":"score","score":0,"confidence":1,"probabilities":{"0":1,"1":0}}`, score},
		{"wrong legend levels", `{"type":"score","score":0,"confidence":1,"legend":{"0":"low","2":"high"},"probabilities":{"0":1,"1":0}}`, score},
		{"null legend description", `{"type":"score","score":0,"confidence":1,"legend":{"0":null,"1":"high"},"probabilities":{"0":1,"1":0}}`, score},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertBadResponse(t, `{"model":"jev","answers":{"ready":`+test.answer+`},"usage":{"input_tokens":0,"output_tokens":0}}`, test.questions)
		})
	}
}

func TestMalformedResponses(t *testing.T) {
	for _, data := range []string{"null", "[]", "{", noulResponse + ` {}`, strings.Replace(noulResponse, `"noul":0`, `"noul":1e999`, 1)} {
		assertBadResponse(t, data, readyQuestions())
	}
}

func TestModelsResponseValidation(t *testing.T) {
	for _, data := range []string{`null`, `{}`, `{"models":null}`, `{"models":[{}]}`, `{"models":[{"name":"jev","description":"model"}]}`} {
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(data)) })
		if _, err := client.ListModels(context.Background()); err == nil {
			t.Errorf("expected invalid models response: %s", data)
		}
	}
}

func TestRetryPolicyRetriesOnlyTemporaryHTTPResponses(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		retries int
		want    int32
	}{
		{"default retries", http.StatusServiceUnavailable, -1, 3},
		{"retries disabled", http.StatusTooManyRequests, 0, 1},
		{"configured retries", 529, 1, 2},
		{"authorization is not retried", http.StatusUnauthorized, -1, 1},
		{"validation is not retried", http.StatusUnprocessableEntity, -1, 1},
		{"other client errors are not retried", http.StatusBadRequest, -1, 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After-Ms", "1")
				w.WriteHeader(test.status)
			}))
			defer server.Close()

			config := jev.Config{APIKey: "test-key", BaseURL: server.URL}
			if test.retries >= 0 {
				config.Retry = &jev.RetryPolicy{MaxRetries: test.retries}
			}
			client, err := jev.New(config)
			if err != nil {
				t.Fatal(err)
			}

			_, err = client.SystemOne(context.Background(), jev.Request{State: "state", Questions: readyQuestions()})
			var apiError *jev.APIError
			if !errors.As(err, &apiError) || apiError.StatusCode != test.status {
				t.Fatalf("error = %#v, want HTTP status %d", err, test.status)
			}
			if got := calls.Load(); got != test.want {
				t.Fatalf("calls = %d, want %d", got, test.want)
			}
		})
	}
}

func TestRetryHonorsRetryAfterAndContext(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := jev.New(jev.Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = client.SystemOne(ctx, jev.Request{State: "state", Questions: readyQuestions()})
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("error = %v, calls = %d", err, calls.Load())
	}
}

func TestRetrySucceedsAfterTemporaryFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After-Ms", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		_, _ = w.Write([]byte(noulResponse))
	}))
	defer server.Close()

	client, err := jev.New(jev.Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.SystemOne(context.Background(), jev.Request{State: "state", Questions: readyQuestions()})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
	if answer, ok := response.Answers["ready"].(jev.NoulAnswer); !ok || answer.Noul != 0 {
		t.Fatalf("answer = %#v", response.Answers["ready"])
	}
}

func TestHTTPErrorExposesValidationFieldsButNotInput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"detail":[{"loc":["body","questions","ready"],"msg":"invalid question","type":"value_error","input":"private prompt"}]}`))
	}))
	defer server.Close()
	client, err := jev.New(jev.Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SystemOne(context.Background(), jev.Request{State: "private prompt", Questions: readyQuestions()})
	var apiError *jev.APIError
	if !errors.As(err, &apiError) || apiError.StatusCode != http.StatusUnprocessableEntity || len(apiError.Details) != 1 || apiError.Details[0].Message != "invalid question" {
		t.Fatalf("error = %#v", err)
	}
	if strings.Contains(err.Error(), "private prompt") {
		t.Fatalf("error reveals request data: %v", err)
	}
}

func TestConcurrentCalls(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(noulResponse))
	})
	const count = 12
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := client.SystemOne(context.Background(), jev.Request{State: "state", Questions: readyQuestions()})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if calls.Load() != count {
		t.Fatalf("calls = %d", calls.Load())
	}
}

func readyQuestions() map[string]jev.Question {
	return map[string]jev.Question{"ready": jev.NoulQuestion{Instructions: "Ready?"}}
}

func readyAnswer(v map[string]any) map[string]any {
	return v["answers"].(map[string]any)["ready"].(map[string]any)
}

func assertBadResponse(t *testing.T, data string, questions map[string]jev.Question) {
	t.Helper()
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(data)) })
	if _, err := client.SystemOne(context.Background(), jev.Request{State: "state", Questions: questions}); err == nil {
		t.Fatal("expected response validation error")
	}
}
