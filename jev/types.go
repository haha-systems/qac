// Package jev provides a client for TypeSafe's System One HTTP API.
// It returns model answers without applying QAC allocation rules.
package jev

import "encoding/json"

// Question is a ChoiceQuestion, ScoreQuestion, or NoulQuestion.
// Both values and pointers are accepted.
type Question interface {
	questionType() string
}

// ChoiceQuestion selects one of the named criteria. Descriptions and instructions
// accept JSON strings, objects, arrays, or nil.
type ChoiceQuestion struct {
	Instructions any            `json:"instructions,omitempty"`
	Criteria     map[string]any `json:"criteria"`
}

func (ChoiceQuestion) questionType() string { return "choice" }

func (q ChoiceQuestion) MarshalJSON() ([]byte, error) {
	type fields ChoiceQuestion
	return json.Marshal(struct {
		Type string `json:"type"`
		fields
	}{Type: "choice", fields: fields(q)})
}

// ScoreQuestion rates state against ordered levels starting at zero.
// Each level accepts a JSON string, object, or array; instructions also accept nil.
type ScoreQuestion struct {
	Instructions any   `json:"instructions,omitempty"`
	Criteria     []any `json:"criteria"`
}

func (ScoreQuestion) questionType() string { return "score" }

func (q ScoreQuestion) MarshalJSON() ([]byte, error) {
	type fields ScoreQuestion
	return json.Marshal(struct {
		Type string `json:"type"`
		fields
	}{Type: "score", fields: fields(q)})
}

// NoulCriteria describes the yes and no outcomes. Both fields accept JSON
// strings, objects, arrays, or nil.
type NoulCriteria struct {
	True  any `json:"true,omitempty"`
	False any `json:"false,omitempty"`
}

// NoulQuestion asks whether a statement is true. Instructions accept a JSON
// string, object, array, or nil. Criteria are optional.
type NoulQuestion struct {
	Instructions any           `json:"instructions,omitempty"`
	Criteria     *NoulCriteria `json:"criteria,omitempty"`
}

func (NoulQuestion) questionType() string { return "noul" }

func (q NoulQuestion) MarshalJSON() ([]byte, error) {
	type fields NoulQuestion
	return json.Marshal(struct {
		Type string `json:"type"`
		fields
	}{Type: "noul", fields: fields(q)})
}

// Request evaluates named questions against shared state. State accepts a JSON
// string, object, or array. An empty Model uses the configured default.
type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

// Answer is a ChoiceAnswer, ScoreAnswer, or NoulAnswer value.
type Answer interface {
	answerType() string
}

// ChoiceAnswer contains the selected option, confidence, and option probabilities.
type ChoiceAnswer struct {
	Choice        string             `json:"choice"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (ChoiceAnswer) answerType() string { return "choice" }

// ScoreAnswer contains a probability-weighted score, not a normalized QAC signal.
// Legend retains structured descriptions keyed by zero-based level numbers.
type ScoreAnswer struct {
	Score         float64            `json:"score"`
	Confidence    float64            `json:"confidence"`
	Legend        map[string]any     `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (ScoreAnswer) answerType() string { return "score" }

// NoulAnswer is the probability of a yes answer, in [0,1].
// The API does not return a separate confidence field for this type.
type NoulAnswer struct {
	Noul float64 `json:"noul"`
}

func (NoulAnswer) answerType() string { return "noul" }

// Usage contains the token counts reported by the API.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Response contains answers by question name and the actual model used.
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Model is a model name or alias available to the account.
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}
