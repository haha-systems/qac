package jevpolicy_test

import (
	"context"
	"os"
	"testing"

	"github.com/haha-systems/qac"
	typesafe "github.com/haha-systems/qac/jev"
	jevpolicy "github.com/haha-systems/qac/policy/jev"
	"github.com/haha-systems/qac/policy/threshold"
)

func TestLiveResourceSelection(t *testing.T) {
	if os.Getenv("JEV_LIVE_TEST") != "1" {
		t.Skip("set JEV_LIVE_TEST=1 to call the TypeSafe API")
	}

	client, err := typesafe.New(typesafe.Config{APIKey: os.Getenv("TYPESAFE_API_KEY")})
	if err != nil {
		t.Fatal(err)
	}

	policy, err := jevpolicy.New(jevpolicy.Config{
		Client:    client,
		Threshold: threshold.Config{Hierarchy: []string{"luna", "sol", "astra"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	request := qac.Request{
		Context: qac.Context{
			CurrentResource: "luna", Uncertainty: .82, Importance: .94,
			Novelty: .66, ExpectedGain: .91, FailedAttempts: 2,
			Metadata: map[string]string{
				"description": "A fictional production incident: after a regional identity-service deploy, about 18% of sign-ins return 503 in one region. The deploy changed a token-cache setting. Error rates rose four minutes later. Application logs show cache timeouts, but database traces are normal. Compare the deploy and cache evidence, identify the likely cause, and recommend a safe rollback or verification step. Do not execute changes. The evidence is incomplete, but the failure is limited to one region and began after a known deploy.",
			},
		},
		Resources: []qac.Resource{
			{
				ID: "luna", Capability: .42, Cost: .12, Scarcity: .08, Available: true,
				Metadata: map[string]string{"description": "Fast, low-cost handling of clear, bounded tasks: summarize evidence, follow a known checklist, and suggest a routine next step. Limited ability to connect weak signals across services."},
			},
			{
				ID: "sol", Capability: .78, Cost: .46, Scarcity: .35, Available: true,
				Metadata: map[string]string{"description": "Strong general reasoning for multi-step debugging across logs, traces, and recent changes. Can compare likely causes and recommend a safe, evidence-based mitigation."},
			},
			{
				ID: "astra", Capability: .96, Cost: .88, Scarcity: .82, Available: true,
				Metadata: map[string]string{"description": "Highest-capability resource for novel, ambiguous, high-impact problems that need broad synthesis, deep research, or a new design. Most costly and scarce."},
			},
		},
		Budget: qac.BudgetState{Resources: map[string]qac.ResourceBudget{
			"luna": {Enabled: true}, "sol": {Enabled: true}, "astra": {Enabled: true},
		}},
	}

	decision, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Model == "" {
		t.Fatalf("Jev did not make a confident choice; threshold fallback selected %q", decision.To)
	}
	if decision.To != "luna" && decision.To != "sol" && decision.To != "astra" {
		t.Fatalf("Jev selected unknown resource %q", decision.To)
	}

	t.Logf("model=%s action=%s resource=%s confidence=%.3f probabilities=%v", decision.Model, decision.Action, decision.To, decision.Confidence, decision.Probabilities)
}
