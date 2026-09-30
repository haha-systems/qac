package jevpolicy_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/jev"
	jevpolicy "github.com/haha-systems/qac/policy/jev"
	"github.com/haha-systems/qac/policy/threshold"
)

func TestJevSelectsEligibleResourceAndReturnsEvidence(t *testing.T) {
	client := &fakeClient{response: choiceResponse("high", .91, .1, .2, .7)}
	policy := mustPolicy(t, client, defaultConfig())

	decision, err := policy.Decide(context.Background(), testRequest("middle"))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != qac.ActionEscalate || decision.From != "middle" || decision.To != "high" {
		t.Fatalf("decision = %#v", decision)
	}
	if decision.Model != "jev-1.13.0" || decision.Confidence != .91 || decision.Score != .91 || decision.Threshold != .5 {
		t.Fatalf("decision evidence = %#v", decision)
	}
	if !reflect.DeepEqual(decision.Probabilities, map[string]float64{"low": .1, "middle": .2, "high": .7}) {
		t.Fatalf("probabilities = %#v", decision.Probabilities)
	}
	if len(decision.Factors) != 1 || decision.Factors[0].Contribution != decision.Score {
		t.Fatalf("factors = %#v, score = %v", decision.Factors, decision.Score)
	}
	if len(decision.Eligibility) != 3 {
		t.Fatalf("eligibility = %#v", decision.Eligibility)
	}
	decision.Probabilities["high"] = 0
	if client.response.Answers["resource"].(jev.ChoiceAnswer).Probabilities["high"] != .7 {
		t.Fatal("decision probabilities alias the client's answer")
	}
}

func TestJevCanSelectCurrentAndNonAdjacentResources(t *testing.T) {
	cases := []struct {
		name, current, selected string
		action                  qac.Action
	}{
		{"continue current", "middle", "middle", qac.ActionContinue},
		{"skip to higher tier", "low", "high", qac.ActionEscalate},
		{"skip to lower tier", "high", "low", qac.ActionRelease},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			probabilities := map[string]float64{"low": .1, "middle": .1, "high": .1}
			probabilities[test.selected] = .8
			client := &fakeClient{response: choiceResponseWithDistribution(test.selected, .9, probabilities)}
			policy := mustPolicy(t, client, defaultConfig())
			decision, err := policy.Decide(context.Background(), testRequest(test.current))
			if err != nil {
				t.Fatal(err)
			}
			if decision.Action != test.action || decision.To != test.selected {
				t.Fatalf("action, destination = %q, %q", decision.Action, decision.To)
			}
		})
	}
}

func TestJevOnlyReceivesEligibleResourcesAndDescriptionMetadata(t *testing.T) {
	client := &fakeClient{response: singleChoiceResponse("low", .9, 1)}
	policy := mustPolicy(t, client, defaultConfig())
	request := testRequest("middle")
	request.Budget.Resources["middle"] = qac.ResourceBudget{Enabled: false}
	request.Resources[2].Available = false

	decision, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != qac.ActionRelease || decision.To != "low" {
		t.Fatalf("decision = %#v", decision)
	}
	if got, want := client.calls, 1; got != want {
		t.Fatalf("Jev calls = %d, want %d", got, want)
	}

	requestJSON, err := json.Marshal(client.req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(requestJSON), "must not be sent") {
		t.Fatalf("private metadata reached Jev: %s", requestJSON)
	}
	if !strings.Contains(string(requestJSON), "task description") || !strings.Contains(string(requestJSON), "low resource") {
		t.Fatalf("descriptions are missing from Jev request: %s", requestJSON)
	}

	question := client.req.Questions["resource"].(jev.ChoiceQuestion)
	if len(question.Criteria) != 1 || question.Criteria["low"] == nil {
		t.Fatalf("choice criteria contain ineligible resources: %#v", question.Criteria)
	}
	stateJSON, err := json.Marshal(client.req.State)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		t.Fatal(err)
	}
	if state["current_resource"] != "middle" || state["uncertainty"] != .8 || state["failed_attempts"] != float64(2) {
		t.Fatalf("task state = %#v", state)
	}
}

func TestNoEligibleResourceSkipsJev(t *testing.T) {
	client := &fakeClient{response: choiceResponse("low", .9, 1, 0, 0)}
	policy := mustPolicy(t, client, defaultConfig())
	request := testRequest("middle")
	request.Resources[0].Available = false
	request.Budget.Resources["middle"] = qac.ResourceBudget{Enabled: false}
	request.Budget.Resources["high"] = qac.ResourceBudget{Enabled: true, RemainingInvocations: intPointer(0)}

	decision, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != qac.ActionStop || decision.To != "" || client.calls != 0 {
		t.Fatalf("decision = %#v, Jev calls = %d", decision, client.calls)
	}
}

func TestTooManyEligibleResourcesUseThresholdFallback(t *testing.T) {
	const count = 256
	hierarchy := make([]string, count)
	resources := make([]qac.Resource, count)
	budgets := make(map[string]qac.ResourceBudget, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("resource-%03d", i)
		hierarchy[i] = id
		resources[i] = qac.Resource{ID: id, Capability: .5, Available: true}
		budgets[id] = qac.ResourceBudget{Enabled: true}
	}

	client := &fakeClient{response: singleChoiceResponse(hierarchy[0], 1, 1)}
	config := jevpolicy.Config{Client: client, Threshold: threshold.Config{Hierarchy: hierarchy}}
	policy, err := jevpolicy.New(config)
	if err != nil {
		t.Fatal(err)
	}
	request := qac.Request{
		Context: qac.Context{CurrentResource: hierarchy[0]}, Resources: resources,
		Budget: qac.BudgetState{Resources: budgets},
	}
	got, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	assertThresholdFallback(t, got, config, request)
	if client.calls != 0 {
		t.Fatalf("Jev calls = %d", client.calls)
	}
}

func TestLowConfidenceAndClientErrorsUseThresholdDecision(t *testing.T) {
	cases := []struct {
		name   string
		client *fakeClient
	}{
		{"low confidence", &fakeClient{response: choiceResponse("high", .49, .1, .2, .7)}},
		{"client error", &fakeClient{err: errors.New("service unavailable")}},
		{"missing answer", &fakeClient{response: jev.Response{Model: "jev-1.13.0", Answers: map[string]jev.Answer{}}}},
		{"missing model", &fakeClient{response: jev.Response{Answers: map[string]jev.Answer{"resource": jev.ChoiceAnswer{Choice: "high", Confidence: .9, Probabilities: map[string]float64{"low": .1, "middle": .2, "high": .7}}}}}},
		{"wrong answer type", &fakeClient{response: jev.Response{Model: "jev-1.13.0", Answers: map[string]jev.Answer{"resource": jev.NoulAnswer{Noul: .9}}}}},
		{"unknown choice", &fakeClient{response: choiceResponse("unknown", .9, .1, .2, .7)}},
		{"invalid confidence", &fakeClient{response: choiceResponse("high", math.NaN(), .1, .2, .7)}},
		{"invalid probability keys", &fakeClient{response: choiceResponseWithDistribution("high", .9, map[string]float64{"low": .1, "middle": .9, "other": 0})}},
		{"choice does not have highest probability", &fakeClient{response: choiceResponse("high", .9, .8, .1, .1)}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			config := defaultConfig()
			policy := mustPolicy(t, test.client, config)
			request := testRequest("middle")
			wantPolicy, err := threshold.New(config.Threshold)
			if err != nil {
				t.Fatal(err)
			}
			want, err := wantPolicy.Decide(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}

			got, err := policy.Decide(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("fallback decision = %#v, want %#v", got, want)
			}
		})
	}
}

func TestConfidenceFloorIsConfigurable(t *testing.T) {
	client := &fakeClient{response: choiceResponse("high", .6, .1, .2, .7)}
	config := defaultConfig()
	config.ConfidenceFloor = .7
	policy := mustPolicy(t, client, config)

	got, err := policy.Decide(context.Background(), testRequest("middle"))
	if err != nil {
		t.Fatal(err)
	}
	wantPolicy, err := threshold.New(config.Threshold)
	if err != nil {
		t.Fatal(err)
	}
	want, err := wantPolicy.Decide(context.Background(), testRequest("middle"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decision = %#v, want threshold fallback %#v", got, want)
	}
}

func TestInternalTimeoutFallsBackAndCallerCancellationReturnsError(t *testing.T) {
	t.Run("internal timeout", func(t *testing.T) {
		client := &fakeClient{wait: true}
		config := defaultConfig()
		config.Timeout = 10 * time.Millisecond
		policy := mustPolicy(t, client, config)
		got, err := policy.Decide(context.Background(), testRequest("middle"))
		if err != nil {
			t.Fatal(err)
		}
		assertThresholdFallback(t, got, config, testRequest("middle"))
	})

	t.Run("caller cancellation", func(t *testing.T) {
		client := &fakeClient{wait: true, started: make(chan struct{})}
		policy := mustPolicy(t, client, defaultConfig())
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := policy.Decide(ctx, testRequest("middle"))
			result <- err
		}()
		<-client.started
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context cancellation", err)
		}
	})
}

func TestInvalidRequestReturnsBeforeJev(t *testing.T) {
	client := &fakeClient{response: choiceResponse("high", .9, .1, .2, .7)}
	policy := mustPolicy(t, client, defaultConfig())
	request := testRequest("middle")
	request.Context.Uncertainty = 1.1
	if _, err := policy.Decide(context.Background(), request); err == nil || client.calls != 0 {
		t.Fatalf("error = %v, Jev calls = %d", err, client.calls)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	client := &fakeClient{response: choiceResponse("high", .9, .1, .2, .7)}
	for _, test := range []struct {
		name string
		edit func(*jevpolicy.Config)
	}{
		{"missing client", func(config *jevpolicy.Config) { config.Client = nil }},
		{"empty hierarchy", func(config *jevpolicy.Config) { config.Threshold.Hierarchy = nil }},
		{"negative confidence floor", func(config *jevpolicy.Config) { config.ConfidenceFloor = -.1 }},
		{"confidence floor above one", func(config *jevpolicy.Config) { config.ConfidenceFloor = 1.1 }},
		{"non-finite confidence floor", func(config *jevpolicy.Config) { config.ConfidenceFloor = math.NaN() }},
		{"negative timeout", func(config *jevpolicy.Config) { config.Timeout = -time.Second }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := defaultConfig()
			config.Client = client
			test.edit(&config)
			if _, err := jevpolicy.New(config); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

type fakeClient struct {
	response jev.Response
	err      error
	req      jev.Request
	calls    int
	wait     bool
	started  chan struct{}
}

func (client *fakeClient) SystemOne(ctx context.Context, request jev.Request) (jev.Response, error) {
	client.calls++
	client.req = request
	if client.wait {
		if client.started != nil {
			close(client.started)
		}
		<-ctx.Done()
		return jev.Response{}, ctx.Err()
	}
	return client.response, client.err
}

func defaultConfig() jevpolicy.Config {
	return jevpolicy.Config{Threshold: threshold.Config{Hierarchy: []string{"low", "middle", "high"}}}
}

func mustPolicy(t *testing.T, client *fakeClient, config jevpolicy.Config) *jevpolicy.Policy {
	t.Helper()
	config.Client = client
	policy, err := jevpolicy.New(config)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func testRequest(current string) qac.Request {
	return qac.Request{
		Context: qac.Context{
			CurrentResource: current, Uncertainty: .8, Importance: .7, Novelty: .6,
			ExpectedGain: .9, FailedAttempts: 2,
			Metadata: map[string]string{"description": "task description", "private": "must not be sent"},
		},
		Resources: []qac.Resource{
			{ID: "low", Capability: .2, Cost: .1, Scarcity: .1, Available: true, Metadata: map[string]string{"description": "low resource", "private": "must not be sent"}},
			{ID: "middle", Capability: .6, Cost: .4, Scarcity: .4, Available: true, Metadata: map[string]string{"description": "middle resource"}},
			{ID: "high", Capability: .95, Cost: .8, Scarcity: .9, Available: true, Metadata: map[string]string{"description": "high resource"}},
		},
		Budget: qac.BudgetState{Resources: map[string]qac.ResourceBudget{
			"low": {Enabled: true}, "middle": {Enabled: true}, "high": {Enabled: true},
		}},
	}
}

func choiceResponse(choice string, confidence, low, middle, high float64) jev.Response {
	return jev.Response{
		Model: "jev-1.13.0",
		Answers: map[string]jev.Answer{"resource": jev.ChoiceAnswer{
			Choice: choice, Confidence: confidence,
			Probabilities: map[string]float64{"low": low, "middle": middle, "high": high},
		}},
	}
}

func singleChoiceResponse(choice string, confidence, probability float64) jev.Response {
	return jev.Response{
		Model: "jev-1.13.0",
		Answers: map[string]jev.Answer{"resource": jev.ChoiceAnswer{
			Choice: choice, Confidence: confidence,
			Probabilities: map[string]float64{choice: probability},
		}},
	}
}

func choiceResponseWithDistribution(choice string, confidence float64, probabilities map[string]float64) jev.Response {
	return jev.Response{
		Model: "jev-1.13.0",
		Answers: map[string]jev.Answer{"resource": jev.ChoiceAnswer{
			Choice: choice, Confidence: confidence, Probabilities: probabilities,
		}},
	}
}

func assertThresholdFallback(t *testing.T, got qac.Decision, config jevpolicy.Config, request qac.Request) {
	t.Helper()
	policy, err := threshold.New(config.Threshold)
	if err != nil {
		t.Fatal(err)
	}
	want, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decision = %#v, want %#v", got, want)
	}
}

func intPointer(value int) *int { return &value }
