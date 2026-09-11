package threshold_test

import (
	"context"
	"testing"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

func TestEligibilityRejectsExhaustedDestination(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade"}})
	zero := 0
	request := requestWithContext("wraith", .8, .8, .8, .8, 3)
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: true, RemainingInvocations: &zero}
	decision, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != qac.ActionContinue || !hasReason(decision.Eligibility, "shade", "invocation budget exhausted") {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestEligibilityReportsAllConstraintReasonsInStableOrder(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade"}})
	request := generatedValidRequest("wraith")
	request.Resources[1].Available = false
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false, Cooldown: true}

	decision := assertAction(t, policy, request, qac.ActionContinue)
	for _, item := range decision.Eligibility {
		if item.ResourceID == "shade" {
			want := []string{"unavailable", "disabled", "cooldown active"}
			if len(item.Reasons) != len(want) {
				t.Fatalf("reasons = %#v, want %#v", item.Reasons, want)
			}
			for i := range want {
				if item.Reasons[i] != want[i] {
					t.Fatalf("reasons = %#v, want %#v", item.Reasons, want)
				}
			}
			return
		}
	}
	t.Fatal("shade eligibility missing")
}

func TestEligibilityRejectsResourceOutsideHierarchy(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade"}})
	decision := assertAction(t, policy, generatedValidRequest("wraith"), qac.ActionContinue)
	if !hasReason(decision.Eligibility, "veil", "not in hierarchy") {
		t.Fatalf("decision = %#v", decision)
	}
}
