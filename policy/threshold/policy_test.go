package threshold_test

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

// These tests fail if scoring uses the source resource, omits a factor, or
// selects a non-adjacent transition.
func TestWraithEscalatesToShade(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	assertDecision(t, policy, requestWithContext("wraith", .72, .50, .40, .76, 2), qac.ActionEscalate, "shade")
}

func TestShadeEscalatesToVeil(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	assertDecision(t, policy, requestWithContext("shade", .91, .86, .78, .93, 3), qac.ActionEscalate, "veil")
}

func TestVeilReleasesToShade(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	assertDecision(t, policy, requestWithContext("veil", .12, .40, .10, .08, 0), qac.ActionRelease, "shade")
}

// This fails if difficulty signals alone bypass the configured threshold.
func TestDifficultyAloneDoesNotForceEscalation(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	assertDecision(t, policy, requestWithContext("shade", 1, 1, 1, 0, 3), qac.ActionContinue, "shade")
}

// This fails if the policy evaluates a score before replacing an ineligible current resource.
func TestCurrentIneligibleUsesNearestEligible(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{
		Hierarchy:           []string{"wraith", "shade", "veil"},
		OnCurrentIneligible: threshold.IneligibleCurrentNearestEligible,
	})
	request := requestWithContext("shade", .5, .5, .5, .5, 0)
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false}
	assertDecision(t, policy, request, qac.ActionRelease, "wraith")
}

// This fails if least-cost selection ignores the destination cost.
func TestCurrentIneligibleUsesLeastCostEligible(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{
		Hierarchy:           []string{"wraith", "shade", "veil"},
		OnCurrentIneligible: threshold.IneligibleCurrentLeastCostEligible,
	})
	request := requestWithContext("shade", .5, .5, .5, .5, 0)
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false}
	decision := assertAction(t, policy, request, qac.ActionRelease)
	if decision.To != "wraith" || !hasReason(decision.Eligibility, "shade", "disabled") {
		t.Fatalf("decision = %#v", decision)
	}
}

// This fails if the least-cost rule does not use cost before hierarchy direction.
func TestCurrentIneligibleLeastCostCanEscalate(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{
		Hierarchy:           []string{"wraith", "shade", "veil"},
		OnCurrentIneligible: threshold.IneligibleCurrentLeastCostEligible,
	})
	request := requestWithContext("shade", .5, .5, .5, .5, 0)
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false}
	request.Resources[0].Cost = .9
	request.Resources[2].Cost = .1
	assertDecision(t, policy, request, qac.ActionEscalate, "veil")
}

// This fails if least-cost ties skip scarcity or hierarchy-index tie-breaking.
func TestCurrentIneligibleLeastCostBreaksTiesDeterministically(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{
		Hierarchy:           []string{"wraith", "shade", "veil"},
		OnCurrentIneligible: threshold.IneligibleCurrentLeastCostEligible,
	})
	request := requestWithContext("shade", .5, .5, .5, .5, 0)
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false}
	request.Resources[0].Cost, request.Resources[2].Cost = .5, .5
	request.Resources[0].Scarcity, request.Resources[2].Scarcity = .9, .1
	assertDecision(t, policy, request, qac.ActionEscalate, "veil")

	request.Resources[0].Scarcity = .1
	assertDecision(t, policy, request, qac.ActionRelease, "wraith")
}

// This fails if the default stop policy silently retains an ineligible current resource.
func TestCurrentIneligibleStops(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	request := generatedValidRequest("shade")
	request.Budget.Resources["shade"] = qac.ResourceBudget{Enabled: false}
	assertDecision(t, policy, request, qac.ActionStop, "")
}

// This fails if replacement mode treats an outside-hierarchy current resource as index zero.
func TestCurrentOutsideHierarchyStopsBeforeReplacement(t *testing.T) {
	for _, mode := range []threshold.IneligibleCurrentPolicy{
		threshold.IneligibleCurrentNearestEligible,
		threshold.IneligibleCurrentLeastCostEligible,
	} {
		t.Run(string(mode), func(t *testing.T) {
			policy := mustPolicy(t, threshold.Config{
				Hierarchy:           []string{"wraith", "shade"},
				OnCurrentIneligible: mode,
			})
			decision := assertAction(t, policy, generatedValidRequest("veil"), qac.ActionStop)
			if decision.To != "" || !hasReason(decision.Eligibility, "veil", "not in hierarchy") {
				t.Fatalf("decision = %#v", decision)
			}
		})
	}
}

// This fails if cancellation is delayed until after request validation.
func TestDecideReturnsCanceledContextUnchanged(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	decision, err := policy.Decide(ctx, qac.Request{})
	if !errors.Is(err, context.Canceled) || err != ctx.Err() {
		t.Fatalf("error = %v, want unchanged context cancellation", err)
	}
	if !reflect.DeepEqual(decision, qac.Decision{}) {
		t.Fatalf("decision = %#v, want empty decision", decision)
	}
}

// This fails if evaluations on adjacent levels can cause an oscillating transition.
func TestHysteresisPreventsOscillation(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	assertDecision(t, policy, requestWithContext("shade", .70, .70, .70, .70, 0), qac.ActionContinue, "shade")
	assertDecision(t, policy, requestWithContext("veil", .70, .70, .70, .70, 0), qac.ActionContinue, "veil")
}

func TestFactorsSumToScore(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	decision, err := policy.Decide(context.Background(), requestWithContext("shade", .5, .5, .5, .5, 1))
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(sum(decision.Factors)-decision.Score) > 1e-12 {
		t.Fatalf("factor sum = %.16f, score = %.16f", sum(decision.Factors), decision.Score)
	}
}

func TestContinueRetainsUnmetUpwardEvaluation(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	decision, err := policy.Decide(context.Background(), requestWithContext("wraith", .5, .5, .5, .5, 1))
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != qac.ActionContinue || decision.To != "wraith" {
		t.Fatalf("decision = %#v", decision)
	}
	if math.Abs(decision.Score-.395) > 1e-12 || decision.Threshold != .55 {
		t.Fatalf("score, threshold = %.16f, %.16f; want .395, .55", decision.Score, decision.Threshold)
	}
	if len(decision.Factors) != 7 || math.Abs(sum(decision.Factors)-decision.Score) > 1e-12 {
		t.Fatalf("factors = %#v, score = %.16f", decision.Factors, decision.Score)
	}
	if decision.Reason != "continue wraith -> wraith: score=0.395000 threshold=0.550000; factors=expected_gain=+0.175000,uncertainty=+0.125000,importance=+0.075000" {
		t.Fatalf("reason = %q", decision.Reason)
	}
}

func TestScoreUsesDestinationCostAndScarcityFactors(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	decision, err := policy.Decide(context.Background(), requestWithContext("wraith", .72, .50, .40, .76, 2))
	if err != nil {
		t.Fatal(err)
	}

	want := []qac.Factor{
		{Name: "uncertainty", Value: .72, Weight: .25, Contribution: .18},
		{Name: "importance", Value: .50, Weight: .15, Contribution: .075},
		{Name: "novelty", Value: .40, Weight: .10, Contribution: .04},
		{Name: "expected_gain", Value: .76, Weight: .35, Contribution: .266},
		{Name: "failures", Value: 2.0 / 3.0, Weight: .15, Contribution: .10},
		{Name: "cost", Value: .35, Weight: .20, Contribution: -.0168},
		{Name: "scarcity", Value: .30, Weight: .30, Contribution: -.0216},
	}
	assertFactors(t, decision.Factors, want)
	if math.Abs(decision.Score-.6226) > 1e-12 {
		t.Fatalf("score = %.16f, want .6226", decision.Score)
	}
}

func TestEscalationWinsWhenBothThresholdsApply(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{
		Hierarchy: []string{"wraith", "shade", "veil"},
		EscalationThresholds: map[threshold.Transition]float64{
			{From: "shade", To: "veil"}: 0,
		},
		ReleaseThresholds: map[threshold.Transition]float64{
			{From: "shade", To: "wraith"}: 1,
		},
	})
	assertDecision(t, policy, requestWithContext("shade", .5, .5, .5, .5, 1), qac.ActionEscalate, "veil")
}

func TestReasonIsDeterministicAndRanksLargestFactors(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	request := requestWithContext("wraith", .72, .50, .40, .76, 2)
	first, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := policy.Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Reason != "escalate wraith -> shade: score=0.622600 threshold=0.550000; factors=expected_gain=+0.266000,uncertainty=+0.180000,failures=+0.100000" {
		t.Fatalf("reason = %q", first.Reason)
	}
	if second.Reason != first.Reason {
		t.Fatalf("non-deterministic reason: %q then %q", first.Reason, second.Reason)
	}
}

func sum(factors []qac.Factor) float64 {
	var total float64
	for _, factor := range factors {
		total += factor.Contribution
	}
	return total
}

func assertFactors(t *testing.T, got, want []qac.Factor) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("factor count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Name != want[index].Name || got[index].Value != want[index].Value || got[index].Weight != want[index].Weight || math.Abs(got[index].Contribution-want[index].Contribution) > 1e-12 {
			t.Fatalf("factor[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}
}
