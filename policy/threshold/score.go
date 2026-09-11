package threshold

import (
	"math"

	"github.com/haha-systems/qac"
)

func (policy *Policy) score(context qac.Context, destination qac.Resource) (float64, []qac.Factor) {
	failures := math.Min(float64(context.FailedAttempts)/float64(policy.failureSaturation), 1)
	factors := []qac.Factor{
		factor("uncertainty", context.Uncertainty, policy.weights.Uncertainty),
		factor("importance", context.Importance, policy.weights.Importance),
		factor("novelty", context.Novelty, policy.weights.Novelty),
		factor("expected_gain", context.ExpectedGain, policy.weights.ExpectedGain),
		factor("failures", failures, policy.weights.Failures),
		negativeFactor("cost", destination.Cost, policy.weights.Cost, context.ExpectedGain),
		negativeFactor("scarcity", destination.Scarcity, policy.weights.Scarcity, context.ExpectedGain),
	}

	var total float64
	for _, item := range factors {
		total += item.Contribution
	}
	return total, factors
}

func factor(name string, value, weight float64) qac.Factor {
	return qac.Factor{Name: name, Value: value, Weight: weight, Contribution: weight * value}
}

func negativeFactor(name string, value, weight, expectedGain float64) qac.Factor {
	return qac.Factor{
		Name:         name,
		Value:        value,
		Weight:       weight,
		Contribution: -weight * value * (1 - expectedGain),
	}
}
