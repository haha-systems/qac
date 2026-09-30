package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/haha-systems/qac"
	typesafe "github.com/haha-systems/qac/jev"
	jevpolicy "github.com/haha-systems/qac/policy/jev"
	"github.com/haha-systems/qac/policy/threshold"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	client, err := typesafe.New(typesafe.Config{APIKey: os.Getenv("TYPESAFE_API_KEY")})
	if err != nil {
		return err
	}

	policy, err := jevpolicy.New(jevpolicy.Config{
		Client:    client,
		Threshold: threshold.Config{Hierarchy: []string{"small", "large"}},
	})
	if err != nil {
		return err
	}

	decision, err := policy.Decide(context.Background(), qac.Request{
		Context: qac.Context{
			CurrentResource: "small", Importance: .7, Uncertainty: .8,
			ExpectedGain: .8, Metadata: map[string]string{"description": "A careful multi-step analysis task."},
		},
		Resources: []qac.Resource{
			{ID: "small", Capability: .4, Cost: .1, Scarcity: .1, Available: true,
				Metadata: map[string]string{"description": "Fast, simple tasks"}},
			{ID: "large", Capability: .9, Cost: .7, Scarcity: .5, Available: true,
				Metadata: map[string]string{"description": "Careful, multi-step analysis"}},
		},
	})
	if err != nil {
		return err
	}

	fmt.Printf("action=%s resource=%s model=%s confidence=%.2f\n", decision.Action, decision.To, decision.Model, decision.Confidence)
	return nil
}
