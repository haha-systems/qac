package threshold

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/haha-systems/qac"
)

// Decide deterministically selects an adjacent eligible resource transition.
func (policy *Policy) Decide(_ context.Context, request qac.Request) (qac.Decision, error) {
	if err := qac.ValidateRequest(request); err != nil {
		return qac.Decision{}, err
	}

	eligibility := policy.eligibility(request)
	decision := qac.Decision{
		Action:      qac.ActionContinue,
		From:        request.Context.CurrentResource,
		To:          request.Context.CurrentResource,
		Eligibility: eligibility,
	}
	if !anyEligible(eligibility) {
		decision.Action = qac.ActionStop
		decision.To = ""
		decision.Reason = "stop " + decision.From + " -> : no eligible resources"
		return decision, nil
	}

	resources := make(map[string]qac.Resource, len(request.Resources))
	eligible := make(map[string]bool, len(eligibility))
	for _, resource := range request.Resources {
		resources[resource.ID] = resource
	}
	for _, item := range eligibility {
		eligible[item.ResourceID] = item.Eligible
	}

	currentIndex := policy.indices[decision.From]
	if currentIndex+1 < len(policy.hierarchy) {
		destination := policy.hierarchy[currentIndex+1]
		score, factors := policy.score(request.Context, resources[destination])
		transition := Transition{From: decision.From, To: destination}
		threshold := policy.escalationThresholds[transition]
		decision.Score, decision.Threshold, decision.Factors = score, threshold, factors
		if score >= threshold {
			if eligible[destination] {
				decision.Action, decision.To = qac.ActionEscalate, destination
			}
			decision.Reason = reason(decision.Action, decision.From, decision.To, score, threshold, factors)
			return decision, nil
		}
	}

	if currentIndex > 0 {
		destination := policy.hierarchy[currentIndex-1]
		score, factors := policy.score(request.Context, resources[destination])
		transition := Transition{From: decision.From, To: destination}
		threshold := policy.releaseThresholds[transition]
		decision.Score, decision.Threshold, decision.Factors = score, threshold, factors
		if score < threshold && eligible[destination] {
			decision.Action, decision.To = qac.ActionRelease, destination
		}
		decision.Reason = reason(decision.Action, decision.From, decision.To, score, threshold, factors)
		return decision, nil
	}

	decision.Reason = reason(decision.Action, decision.From, decision.To, decision.Score, decision.Threshold, decision.Factors)
	return decision, nil
}

func reason(action qac.Action, from, to string, score, threshold float64, factors []qac.Factor) string {
	ranked := append([]qac.Factor(nil), factors...)
	sort.Slice(ranked, func(left, right int) bool {
		leftContribution := math.Abs(ranked[left].Contribution)
		rightContribution := math.Abs(ranked[right].Contribution)
		if leftContribution == rightContribution {
			return ranked[left].Name < ranked[right].Name
		}
		return leftContribution > rightContribution
	})
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}

	parts := make([]string, len(ranked))
	for index, item := range ranked {
		parts[index] = fmt.Sprintf("%s=%+.6f", item.Name, item.Contribution)
	}
	return fmt.Sprintf("%s %s -> %s: score=%.6f threshold=%.6f; factors=%s", action, from, to, score, threshold, strings.Join(parts, ","))
}
