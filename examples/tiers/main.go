// Command tiers shows adjacent allocation decisions for a three-level hierarchy.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/haha-systems/qac"
	"github.com/haha-systems/qac/policy/threshold"
)

func main() {
	policy, err := threshold.New(threshold.Config{Hierarchy: []string{"wraith", "shade", "veil"}})
	if err != nil {
		log.Fatal(err)
	}

	for _, request := range []qac.Request{
		requestFor("wraith", .72, .50, .40, .76, 2),
		requestFor("shade", .91, .86, .78, .93, 3),
		requestFor("veil", .12, .40, .10, .08, 0),
	} {
		decision, err := policy.Decide(context.Background(), request)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s %s -> %s\n", decision.Action, decision.From, decision.To)
	}
}

func requestFor(current string, uncertainty, importance, novelty, expectedGain float64, failedAttempts int) qac.Request {
	return qac.Request{
		Context: qac.Context{CurrentResource: current, Uncertainty: uncertainty, Importance: importance, Novelty: novelty, ExpectedGain: expectedGain, FailedAttempts: failedAttempts},
		Resources: []qac.Resource{
			{ID: "wraith", Capability: .30, Cost: .10, Scarcity: .05, Available: true},
			{ID: "shade", Capability: .65, Cost: .35, Scarcity: .30, Available: true},
			{ID: "veil", Capability: .95, Cost: .80, Scarcity: .95, Available: true},
		},
		Budget: qac.BudgetState{Resources: map[string]qac.ResourceBudget{}},
	}
}
