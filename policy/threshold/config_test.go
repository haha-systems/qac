package threshold_test

import (
	"math"
	"testing"

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
