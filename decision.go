package qac

// Factor is a weighted score component.
type Factor struct {
	Name                        string
	Value, Weight, Contribution float64
}

// Eligibility reports whether a resource can receive work.
type Eligibility struct {
	ResourceID string
	Eligible   bool
	Reasons    []string
}

// Decision is the result of a policy evaluation.
type Decision struct {
	Action           Action
	From, To         string
	Score, Threshold float64
	Factors          []Factor
	Reason           string
	Eligibility      []Eligibility
	Model            string             // Model that made a probabilistic choice; empty for deterministic decisions.
	Confidence       float64            // Confidence for the selected choice; zero when the policy has none.
	Probabilities    map[string]float64 // Choice probabilities keyed by resource ID; nil when unavailable.
}
