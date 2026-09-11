package threshold_test

import (
	"context"
	"testing"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

func testResources() []qac.Resource {
	return []qac.Resource{
		{ID: "wraith", Capability: .30, Cost: .10, Scarcity: .05, Available: true},
		{ID: "shade", Capability: .65, Cost: .35, Scarcity: .30, Available: true},
		{ID: "veil", Capability: .95, Cost: .80, Scarcity: .95, Available: true},
	}
}

func requestWithContext(current string, uncertainty, importance, novelty, expectedGain float64, failures int) qac.Request {
	return qac.Request{
		Context:   qac.Context{CurrentResource: current, Uncertainty: uncertainty, Importance: importance, Novelty: novelty, ExpectedGain: expectedGain, FailedAttempts: failures},
		Resources: testResources(),
		Budget:    qac.BudgetState{Resources: map[string]qac.ResourceBudget{}},
	}
}

func mustPolicy(t *testing.T, config threshold.Config) *threshold.Policy {
	t.Helper()
	policy, err := threshold.New(config)
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func assertAction(t *testing.T, policy *threshold.Policy, request qac.Request, want qac.Action) qac.Decision {
	t.Helper()
	decision, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != want {
		t.Fatalf("action = %q, want %q", decision.Action, want)
	}
	return decision
}

func assertDecision(t *testing.T, policy *threshold.Policy, request qac.Request, wantAction qac.Action, wantTo string) {
	t.Helper()
	decision := assertAction(t, policy, request, wantAction)
	if decision.To != wantTo {
		t.Fatalf("destination = %q, want %q", decision.To, wantTo)
	}
}

func eligible(t *testing.T, policy *threshold.Policy, request qac.Request, resourceID string) bool {
	t.Helper()
	for _, item := range assertAction(t, policy, request, qac.ActionContinue).Eligibility {
		if item.ResourceID == resourceID {
			return item.Eligible
		}
	}
	t.Fatalf("eligibility is missing resource %q", resourceID)
	return false
}

func generatedValidRequest(current string) qac.Request {
	return requestWithContext(current, .5, .5, .5, .5, 0)
}

func hasReason(items []qac.Eligibility, id, want string) bool {
	for _, item := range items {
		if item.ResourceID != id {
			continue
		}
		for _, reason := range item.Reasons {
			if reason == want {
				return true
			}
		}
	}
	return false
}
