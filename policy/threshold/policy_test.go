package threshold_test

import (
	"context"
	"math"
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
