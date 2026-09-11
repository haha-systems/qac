package threshold_test

import (
	"context"
	"math"
	"testing"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

func FuzzDecisionNeverSelectsIneligible(f *testing.F) {
	f.Add(uint8(0), uint8(1), uint8(2))
	policies := make([]*threshold.Policy, 0, 3)
	for _, mode := range []threshold.IneligibleCurrentPolicy{
		threshold.IneligibleCurrentStop,
		threshold.IneligibleCurrentNearestEligible,
		threshold.IneligibleCurrentLeastCostEligible,
	} {
		policy, err := threshold.New(threshold.Config{
			Hierarchy:           []string{"wraith", "shade", "veil"},
			OnCurrentIneligible: mode,
		})
		if err != nil {
			f.Fatal(err)
		}
		policies = append(policies, policy)
	}
	f.Fuzz(func(t *testing.T, unavailable, disabled, cooldown uint8) {
		for _, policy := range policies {
			decision, err := policy.Decide(context.Background(), generatedFuzzRequest(unavailable, disabled, cooldown))
			if err != nil {
				t.Fatal(err)
			}
			if decision.To != "" && !decisionEligible(decision.Eligibility, decision.To) {
				t.Fatalf("selected ineligible %q", decision.To)
			}
			if math.IsNaN(decision.Score) || math.IsInf(decision.Score, 0) {
				t.Fatal("non-finite score")
			}
		}
	})
}

func generatedFuzzRequest(unavailable, disabled, cooldown uint8) qac.Request {
	request := generatedValidRequest("shade")
	for index := range request.Resources {
		mask := uint8(1 << index)
		request.Resources[index].Available = unavailable&mask == 0
		if disabled&mask != 0 || cooldown&mask != 0 {
			request.Budget.Resources[request.Resources[index].ID] = qac.ResourceBudget{
				Enabled:  disabled&mask == 0,
				Cooldown: cooldown&mask != 0,
			}
		}
	}
	return request
}

func decisionEligible(items []qac.Eligibility, resourceID string) bool {
	for _, item := range items {
		if item.ResourceID == resourceID {
			return item.Eligible
		}
	}
	return false
}
