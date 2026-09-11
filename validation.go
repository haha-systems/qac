package qac

import (
	"fmt"
	"math"
)

// ValidateRequest checks that a request is safe for deterministic evaluation.
func ValidateRequest(request Request) error {
	if len(request.Resources) == 0 {
		return fmt.Errorf("request must include at least one resource")
	}

	resourceIDs := make(map[string]struct{}, len(request.Resources))
	for _, resource := range request.Resources {
		if resource.ID == "" {
			return fmt.Errorf("resource ID must not be empty")
		}
		if _, found := resourceIDs[resource.ID]; found {
			return fmt.Errorf("duplicate resource ID %q", resource.ID)
		}
		resourceIDs[resource.ID] = struct{}{}

		if err := validateNormalized("resource capability", resource.Capability); err != nil {
			return err
		}
		if err := validateNormalized("resource cost", resource.Cost); err != nil {
			return err
		}
		if err := validateNormalized("resource scarcity", resource.Scarcity); err != nil {
			return err
		}
	}

	if request.Context.CurrentResource == "" {
		return fmt.Errorf("current resource must not be empty")
	}
	if _, found := resourceIDs[request.Context.CurrentResource]; !found {
		return fmt.Errorf("current resource %q is absent", request.Context.CurrentResource)
	}

	for name, value := range map[string]float64{
		"uncertainty":   request.Context.Uncertainty,
		"importance":    request.Context.Importance,
		"novelty":       request.Context.Novelty,
		"expected gain": request.Context.ExpectedGain,
	} {
		if err := validateNormalized(name, value); err != nil {
			return err
		}
	}
	if request.Context.FailedAttempts < 0 {
		return fmt.Errorf("failed attempts must not be negative")
	}

	for resourceID, budget := range request.Budget.Resources {
		if _, found := resourceIDs[resourceID]; !found {
			return fmt.Errorf("budget references unknown resource %q", resourceID)
		}
		if budget.RemainingInvocations != nil && *budget.RemainingInvocations < 0 {
			return fmt.Errorf("remaining invocations for resource %q must not be negative", resourceID)
		}
	}

	return nil
}

func validateNormalized(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return fmt.Errorf("%s must be a finite value in [0,1]", name)
	}
	return nil
}
