package threshold_test

import (
	"context"
	"math"
	"testing"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

func TestConfigRejectsDuplicateHierarchyID(t *testing.T) {
	_, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith", "wraith"}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigRejectsEmptyHierarchyID(t *testing.T) {
	_, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith", ""}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigRejectsInvalidPolicy(t *testing.T) {
	_, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith"}, OnCurrentIneligible: "invalid"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigRejectsNonFiniteAndNegativeWeights(t *testing.T) {
	for _, weight := range []float64{math.NaN(), math.Inf(1), -.01} {
		_, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith"}, Weights: threshold.Weights{Cost: weight}})
		if err == nil {
			t.Fatalf("weight %v: expected error", weight)
		}
	}
}

func TestConfigRejectsInvalidThreshold(t *testing.T) {
	for _, value := range []float64{-.01, 1.01, math.NaN(), math.Inf(1)} {
		_, err := threshold.New(threshold.Config{
			Hierarchy:            []string{"wraith", "shade"},
			EscalationThresholds: map[threshold.Transition]float64{{From: "wraith", To: "shade"}: value},
		})
		if err == nil {
			t.Fatalf("threshold %v: expected error", value)
		}
	}
}

func TestConfigRejectsNonAdjacentTransition(t *testing.T) {
	_, err := threshold.New(threshold.Config{
		Hierarchy:            []string{"wraith", "shade", "veil"},
		EscalationThresholds: map[threshold.Transition]float64{{From: "wraith", To: "veil"}: .5},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConfigRejectsNonPositiveExplicitSaturation(t *testing.T) {
	_, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith"}, FailureSaturation: -1})
	if err == nil {
		t.Fatal("expected error")
	}
}

// This fails if the default thresholds no longer preserve their escalation and release bands.
func TestDefaultThresholdsPreserveEscalationAndReleaseBands(t *testing.T) {
	policy := mustPolicy(t, threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	cases := []struct {
		name      string
		request   qac.Request
		want      qac.Action
		threshold float64
	}{
		{"first escalation", requestWithContext("wraith", .5, .5, .5, .5, 1), qac.ActionContinue, .55},
		{"upper escalation", requestWithContext("shade", .91, .86, .78, .93, 3), qac.ActionEscalate, .80},
		{"first release", requestWithContext("shade", .12, .40, .10, .08, 0), qac.ActionRelease, .25},
		{"upper release", requestWithContext("veil", .12, .40, .10, .08, 0), qac.ActionRelease, .45},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			decision, err := policy.Decide(context.Background(), test.request)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Action != test.want || decision.Threshold != test.threshold {
				t.Fatalf("decision = %#v, want action %q and threshold %.2f", decision, test.want, test.threshold)
			}
		})
	}
}

// This fails if New retains caller-owned hierarchy or threshold maps.
func TestPolicyDefensivelyCopiesConfiguration(t *testing.T) {
	hierarchy := []string{"wraith", "shade", "veil"}
	escalation := map[threshold.Transition]float64{{From: "wraith", To: "shade"}: 1}
	release := map[threshold.Transition]float64{{From: "veil", To: "shade"}: 0}
	policy := mustPolicy(t, threshold.Config{
		Hierarchy:            hierarchy,
		EscalationThresholds: escalation,
		ReleaseThresholds:    release,
	})

	hierarchy[1] = "corrupted"
	escalation[threshold.Transition{From: "wraith", To: "shade"}] = 0
	release[threshold.Transition{From: "veil", To: "shade"}] = 1

	assertDecision(t, policy, requestWithContext("wraith", .72, .50, .40, .76, 2), qac.ActionContinue, "wraith")
	assertDecision(t, policy, requestWithContext("veil", .5, .5, .5, .5, 0), qac.ActionContinue, "veil")
}
