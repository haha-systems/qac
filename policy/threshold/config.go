// Package threshold provides a configurable, deterministic threshold policy.
package threshold

import (
	"fmt"
	"math"
)

// Transition identifies a directed hierarchy transition.
type Transition struct {
	From string
	To   string
}

// Weights control each score contribution.
type Weights struct {
	Uncertainty  float64
	Importance   float64
	Novelty      float64
	ExpectedGain float64
	Failures     float64
	Cost         float64
	Scarcity     float64
}

// IneligibleCurrentPolicy controls handling of an ineligible current resource.
type IneligibleCurrentPolicy string

const (
	IneligibleCurrentStop              IneligibleCurrentPolicy = "stop"
	IneligibleCurrentNearestEligible   IneligibleCurrentPolicy = "nearest_eligible"
	IneligibleCurrentLeastCostEligible IneligibleCurrentPolicy = "least_cost_eligible"
)

// Config configures a threshold Policy.
type Config struct {
	Hierarchy            []string
	Weights              Weights
	EscalationThresholds map[Transition]float64
	ReleaseThresholds    map[Transition]float64
	FailureSaturation    int
	OnCurrentIneligible  IneligibleCurrentPolicy
}

// Policy is an immutable threshold policy.
type Policy struct {
	hierarchy            []string
	indices              map[string]int
	weights              Weights
	escalationThresholds map[Transition]float64
	releaseThresholds    map[Transition]float64
	failureSaturation    int
	onCurrentIneligible  IneligibleCurrentPolicy
}

var defaultWeights = Weights{
	Uncertainty:  .25,
	Importance:   .15,
	Novelty:      .10,
	ExpectedGain: .35,
	Failures:     .15,
	Cost:         .20,
	Scarcity:     .30,
}

// New creates an immutable threshold policy from config.
func New(config Config) (*Policy, error) {
	if len(config.Hierarchy) == 0 {
		return nil, fmt.Errorf("hierarchy must include at least one resource")
	}

	indices := make(map[string]int, len(config.Hierarchy))
	hierarchy := make([]string, len(config.Hierarchy))
	for index, id := range config.Hierarchy {
		if id == "" {
			return nil, fmt.Errorf("hierarchy ID must not be empty")
		}
		if _, exists := indices[id]; exists {
			return nil, fmt.Errorf("duplicate hierarchy ID %q", id)
		}
		indices[id] = index
		hierarchy[index] = id
	}

	if config.OnCurrentIneligible != "" && config.OnCurrentIneligible != IneligibleCurrentStop && config.OnCurrentIneligible != IneligibleCurrentNearestEligible && config.OnCurrentIneligible != IneligibleCurrentLeastCostEligible {
		return nil, fmt.Errorf("invalid current ineligible policy %q", config.OnCurrentIneligible)
	}
	if err := validateWeights(config.Weights); err != nil {
		return nil, err
	}
	if config.FailureSaturation < 0 {
		return nil, fmt.Errorf("failure saturation must be positive")
	}
	if err := validateThresholds(config.EscalationThresholds, indices, 1, "escalation"); err != nil {
		return nil, err
	}
	if err := validateThresholds(config.ReleaseThresholds, indices, -1, "release"); err != nil {
		return nil, err
	}

	weights := config.Weights
	if weights == (Weights{}) {
		weights = defaultWeights
	}
	failureSaturation := config.FailureSaturation
	if failureSaturation == 0 {
		failureSaturation = 3
	}
	currentPolicy := config.OnCurrentIneligible
	if currentPolicy == "" {
		currentPolicy = IneligibleCurrentStop
	}

	escalation := copyThresholds(config.EscalationThresholds)
	if escalation == nil {
		escalation = defaultThresholds(hierarchy, 1)
	}
	release := copyThresholds(config.ReleaseThresholds)
	if release == nil {
		release = defaultThresholds(hierarchy, -1)
	}

	return &Policy{
		hierarchy:            hierarchy,
		indices:              indices,
		weights:              weights,
		escalationThresholds: escalation,
		releaseThresholds:    release,
		failureSaturation:    failureSaturation,
		onCurrentIneligible:  currentPolicy,
	}, nil
}

func validateWeights(weights Weights) error {
	for _, item := range []struct {
		name  string
		value float64
	}{
		{"uncertainty weight", weights.Uncertainty},
		{"importance weight", weights.Importance},
		{"novelty weight", weights.Novelty},
		{"expected gain weight", weights.ExpectedGain},
		{"failures weight", weights.Failures},
		{"cost weight", weights.Cost},
		{"scarcity weight", weights.Scarcity},
	} {
		if math.IsNaN(item.value) || math.IsInf(item.value, 0) || item.value < 0 {
			return fmt.Errorf("%s must be finite and non-negative", item.name)
		}
	}
	return nil
}

func validateThresholds(thresholds map[Transition]float64, indices map[string]int, direction int, name string) error {
	for transition, threshold := range thresholds {
		from, fromFound := indices[transition.From]
		to, toFound := indices[transition.To]
		if !fromFound || !toFound || to-from != direction {
			return fmt.Errorf("invalid %s transition %q to %q", name, transition.From, transition.To)
		}
		if math.IsNaN(threshold) || math.IsInf(threshold, 0) || threshold < 0 || threshold > 1 {
			return fmt.Errorf("%s threshold must be a finite value in [0,1]", name)
		}
	}
	return nil
}

func copyThresholds(thresholds map[Transition]float64) map[Transition]float64 {
	if thresholds == nil {
		return nil
	}
	result := make(map[Transition]float64, len(thresholds))
	for transition, value := range thresholds {
		result[transition] = value
	}
	return result
}

func defaultThresholds(hierarchy []string, direction int) map[Transition]float64 {
	thresholds := make(map[Transition]float64, len(hierarchy)-1)
	for index := 0; index+1 < len(hierarchy); index++ {
		if direction == 1 {
			value := .55
			if index > 0 {
				value = .80
			}
			thresholds[Transition{From: hierarchy[index], To: hierarchy[index+1]}] = value
			continue
		}
		value := .25
		if index > 0 {
			value = .45
		}
		thresholds[Transition{From: hierarchy[index+1], To: hierarchy[index]}] = value
	}
	return thresholds
}
