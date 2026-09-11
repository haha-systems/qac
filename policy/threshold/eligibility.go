package threshold

import "github.com/haha-systems/qac"

func (policy *Policy) eligibility(request qac.Request) []qac.Eligibility {
	result := make([]qac.Eligibility, 0, len(request.Resources))
	for _, resource := range request.Resources {
		reasons := make([]string, 0, 5)
		if !resource.Available {
			reasons = append(reasons, "unavailable")
		}
		if _, found := policy.indices[resource.ID]; !found {
			reasons = append(reasons, "not in hierarchy")
		}
		if budget, found := request.Budget.Resources[resource.ID]; found {
			if !budget.Enabled {
				reasons = append(reasons, "disabled")
			}
			if budget.Cooldown {
				reasons = append(reasons, "cooldown active")
			}
			if budget.RemainingInvocations != nil && *budget.RemainingInvocations == 0 {
				reasons = append(reasons, "invocation budget exhausted")
			}
		}
		result = append(result, qac.Eligibility{ResourceID: resource.ID, Eligible: len(reasons) == 0, Reasons: reasons})
	}
	return result
}

func anyEligible(items []qac.Eligibility) bool {
	for _, item := range items {
		if item.Eligible {
			return true
		}
	}
	return false
}
