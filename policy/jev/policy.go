// Package jev selects eligible QAC resources with TypeSafe's Jev API.
package jevpolicy

import (
	"context"
	"fmt"
	"maps"
	"math"
	"time"

	"github.com/haha-systems/qac"
	typesafe "github.com/haha-systems/qac/jev"
	"github.com/haha-systems/qac/policy/threshold"
)

const (
	defaultConfidenceFloor = .5
	defaultTimeout         = 5 * time.Second
	choiceName             = "resource"
	maxChoiceOptions       = 255
)

const choiceInstructions = "Choose the eligible cognitive resource best suited to this task. Consider the task description, QAC signals, resource descriptions, capability, cost, and scarcity. Choose one listed resource."

// Client is the part of the Jev client used to select a resource.
type Client interface {
	SystemOne(context.Context, typesafe.Request) (typesafe.Response, error)
}

// Config sets the Jev client, the deterministic fallback policy, and call limits.
type Config struct {
	Client          Client
	Threshold       threshold.Config
	ConfidenceFloor float64
	Timeout         time.Duration
}

// Policy asks Jev to choose an eligible resource and uses the threshold policy
// when Jev is unavailable or uncertain.
type Policy struct {
	client          Client
	fallback        *threshold.Policy
	confidenceFloor float64
	timeout         time.Duration
	indices         map[string]int
}

// New creates a policy with the configured hierarchy and its threshold fallback.
func New(config Config) (*Policy, error) {
	if config.Client == nil {
		return nil, fmt.Errorf("Jev client is required")
	}

	fallback, err := threshold.New(config.Threshold)
	if err != nil {
		return nil, err
	}

	floor := config.ConfidenceFloor
	if floor == 0 {
		floor = defaultConfidenceFloor
	}
	if math.IsNaN(floor) || math.IsInf(floor, 0) || floor < 0 || floor > 1 {
		return nil, fmt.Errorf("confidence floor must be a finite value in [0,1]")
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	if timeout < 0 {
		return nil, fmt.Errorf("timeout must be positive")
	}

	indices := make(map[string]int, len(config.Threshold.Hierarchy))
	for index, id := range config.Threshold.Hierarchy {
		indices[id] = index
	}

	return &Policy{
		client: config.Client, fallback: fallback, confidenceFloor: floor,
		timeout: timeout, indices: indices,
	}, nil
}

// Decide returns Jev's resource choice when it is eligible and sufficiently
// confident. Invalid requests and caller cancellation remain errors.
func (policy *Policy) Decide(ctx context.Context, request qac.Request) (qac.Decision, error) {
	fallback, err := policy.fallback.Decide(ctx, request)
	if err != nil {
		return qac.Decision{}, err
	}
	if err := ctx.Err(); err != nil {
		return qac.Decision{}, err
	}

	resources := eligibleResources(request, fallback.Eligibility)
	if len(resources) == 0 || len(resources) > maxChoiceOptions {
		return fallback, nil
	}
	currentIndex, currentFound := policy.indices[request.Context.CurrentResource]
	if !currentFound {
		return fallback, nil
	}
	if err := ctx.Err(); err != nil {
		return qac.Decision{}, err
	}

	jevContext, cancel := context.WithTimeout(ctx, policy.timeout)
	response, err := policy.client.SystemOne(jevContext, selectionRequest(request, resources))
	cancel()
	if callerErr := ctx.Err(); callerErr != nil {
		return qac.Decision{}, callerErr
	}
	if err != nil {
		return fallback, nil
	}

	answer, ok := choiceAnswer(response)
	if !ok || response.Model == "" || answer.Confidence < policy.confidenceFloor || !validAnswer(answer, resources) {
		return fallback, nil
	}

	selectedIndex := policy.indices[answer.Choice]
	action := qac.ActionContinue
	if selectedIndex > currentIndex {
		action = qac.ActionEscalate
	} else if selectedIndex < currentIndex {
		action = qac.ActionRelease
	}

	return qac.Decision{
		Action: action, From: request.Context.CurrentResource, To: answer.Choice,
		Score: answer.Confidence, Threshold: policy.confidenceFloor,
		Factors:     []qac.Factor{{Name: "confidence", Value: answer.Confidence, Weight: 1, Contribution: answer.Confidence}},
		Reason:      fmt.Sprintf("jev selected %s with confidence %.6f", answer.Choice, answer.Confidence),
		Eligibility: fallback.Eligibility, Model: response.Model,
		Confidence: answer.Confidence, Probabilities: copyProbabilities(answer.Probabilities),
	}, nil
}

type taskState struct {
	Description     string  `json:"description,omitempty"`
	CurrentResource string  `json:"current_resource"`
	Uncertainty     float64 `json:"uncertainty"`
	Importance      float64 `json:"importance"`
	Novelty         float64 `json:"novelty"`
	ExpectedGain    float64 `json:"expected_gain"`
	FailedAttempts  int     `json:"failed_attempts"`
}

type resourceOption struct {
	Description string  `json:"description,omitempty"`
	Capability  float64 `json:"capability"`
	Cost        float64 `json:"cost"`
	Scarcity    float64 `json:"scarcity"`
}

func selectionRequest(request qac.Request, resources map[string]qac.Resource) typesafe.Request {
	context := request.Context
	state := taskState{
		Description: context.Metadata["description"], CurrentResource: context.CurrentResource,
		Uncertainty: context.Uncertainty, Importance: context.Importance,
		Novelty: context.Novelty, ExpectedGain: context.ExpectedGain,
		FailedAttempts: context.FailedAttempts,
	}
	criteria := make(map[string]any, len(resources))
	for id, resource := range resources {
		criteria[id] = resourceOption{
			Description: resource.Metadata["description"], Capability: resource.Capability,
			Cost: resource.Cost, Scarcity: resource.Scarcity,
		}
	}

	return typesafe.Request{
		State: state,
		Questions: map[string]typesafe.Question{
			choiceName: typesafe.ChoiceQuestion{Instructions: choiceInstructions, Criteria: criteria},
		},
	}
}

func eligibleResources(request qac.Request, eligibility []qac.Eligibility) map[string]qac.Resource {
	resourcesByID := make(map[string]qac.Resource, len(request.Resources))
	for _, resource := range request.Resources {
		resourcesByID[resource.ID] = resource
	}

	eligible := make(map[string]qac.Resource, len(eligibility))
	for _, item := range eligibility {
		if item.Eligible {
			if resource, found := resourcesByID[item.ResourceID]; found {
				eligible[item.ResourceID] = resource
			}
		}
	}

	return eligible
}

func choiceAnswer(response typesafe.Response) (typesafe.ChoiceAnswer, bool) {
	if len(response.Answers) != 1 {
		return typesafe.ChoiceAnswer{}, false
	}

	answer, found := response.Answers[choiceName]
	if !found {
		return typesafe.ChoiceAnswer{}, false
	}

	switch typed := answer.(type) {
	case typesafe.ChoiceAnswer:
		return typed, true
	case *typesafe.ChoiceAnswer:
		if typed != nil {
			return *typed, true
		}
	}

	return typesafe.ChoiceAnswer{}, false
}

func validAnswer(answer typesafe.ChoiceAnswer, resources map[string]qac.Resource) bool {
	if answer.Choice == "" || math.IsNaN(answer.Confidence) || math.IsInf(answer.Confidence, 0) || answer.Confidence < 0 || answer.Confidence > 1 {
		return false
	}
	if _, found := resources[answer.Choice]; !found || len(answer.Probabilities) != len(resources) {
		return false
	}

	var total float64
	chosenProbability := answer.Probabilities[answer.Choice]
	for id := range resources {
		probability, found := answer.Probabilities[id]
		if !found || math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 {
			return false
		}
		if probability > chosenProbability {
			return false
		}
		total += probability
	}

	return math.Abs(total-1) <= .02
}

func copyProbabilities(probabilities map[string]float64) map[string]float64 {
	copy := make(map[string]float64, len(probabilities))
	maps.Copy(copy, probabilities)
	return copy
}
