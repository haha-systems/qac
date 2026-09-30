package jev

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

func decodeResponse(data []byte, questions map[string]json.RawMessage) (Response, error) {
	fields, err := object(data, "model", "answers", "usage")
	if err != nil {
		return Response{}, err
	}

	var result Response
	if err := json.Unmarshal(fields["model"], &result.Model); err != nil || result.Model == "" {
		return Response{}, fmt.Errorf("jev: invalid response model")
	}

	if _, err := object(fields["usage"], "input_tokens", "output_tokens"); err != nil {
		return Response{}, err
	}
	if err := json.Unmarshal(fields["usage"], &result.Usage); err != nil || result.Usage.InputTokens < 0 || result.Usage.OutputTokens < 0 {
		return Response{}, fmt.Errorf("jev: invalid token usage")
	}

	answers, err := object(fields["answers"])
	if err != nil || len(answers) != len(questions) {
		return Response{}, fmt.Errorf("jev: response answers do not match questions")
	}

	result.Answers = make(map[string]Answer, len(questions))
	for name, question := range questions {
		raw, found := answers[name]
		if !found {
			return Response{}, fmt.Errorf("jev: missing answer")
		}

		answer, err := decodeAnswer(raw, question)
		if err != nil {
			return Response{}, err
		}

		result.Answers[name] = answer
	}

	return result, nil
}

func decodeAnswer(raw, question json.RawMessage) (Answer, error) {
	fields, err := object(raw, "type")
	if err != nil {
		return nil, err
	}

	var kind string
	if err := json.Unmarshal(fields["type"], &kind); err != nil {
		return nil, fmt.Errorf("jev: invalid answer type")
	}

	questionFields, _ := object(question, "type")
	var expectedKind string
	_ = json.Unmarshal(questionFields["type"], &expectedKind)
	if kind != expectedKind {
		return nil, fmt.Errorf("jev: answer type does not match question")
	}

	switch kind {
	case "choice":
		if _, err := object(raw, "choice", "confidence", "probabilities"); err != nil {
			return nil, err
		}

		var answer ChoiceAnswer
		if err := json.Unmarshal(raw, &answer); err != nil || !normalized(answer.Confidence) {
			return nil, fmt.Errorf("jev: invalid choice answer")
		}

		criteria, _ := object(questionFields["criteria"])
		if _, found := criteria[answer.Choice]; !found {
			return nil, fmt.Errorf("jev: selected choice was not requested")
		}
		if err := validateProbabilities(fields["probabilities"], criteria); err != nil {
			return nil, err
		}

		return answer, nil
	case "score":
		if _, err := object(raw, "score", "confidence", "legend", "probabilities"); err != nil {
			return nil, err
		}

		var answer ScoreAnswer
		if err := json.Unmarshal(raw, &answer); err != nil || !normalized(answer.Confidence) || math.IsNaN(answer.Score) || math.IsInf(answer.Score, 0) {
			return nil, fmt.Errorf("jev: invalid score answer")
		}

		var levels []json.RawMessage
		_ = json.Unmarshal(questionFields["criteria"], &levels)
		if answer.Score < 0 || answer.Score > float64(len(levels)-1) || len(answer.Legend) != len(levels) {
			return nil, fmt.Errorf("jev: score or legend is outside requested levels")
		}

		expected := make(map[string]json.RawMessage, len(levels))
		legend, _ := object(fields["legend"])
		for i := range levels {
			key := strconv.Itoa(i)
			value, found := legend[key]
			if !found || description(value, false) != nil {
				return nil, fmt.Errorf("jev: invalid score legend")
			}

			expected[key] = nil
		}

		if err := validateProbabilities(fields["probabilities"], expected); err != nil {
			return nil, err
		}

		return answer, nil
	case "noul":
		if _, err := object(raw, "noul"); err != nil {
			return nil, err
		}

		var answer NoulAnswer
		if err := json.Unmarshal(raw, &answer); err != nil || !normalized(answer.Noul) {
			return nil, fmt.Errorf("jev: invalid yes/no answer")
		}

		return answer, nil
	default:
		return nil, fmt.Errorf("jev: unknown answer type")
	}
}

func decodeModels(data []byte) ([]Model, error) {
	fields, err := object(data, "models")
	if err != nil {
		return nil, err
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(fields["models"], &entries); err != nil || entries == nil {
		return nil, fmt.Errorf("jev: invalid models list")
	}

	models := make([]Model, 0, len(entries))
	for _, raw := range entries {
		if _, err := object(raw, "name", "description", "release_date"); err != nil {
			return nil, err
		}

		var model Model
		if err := json.Unmarshal(raw, &model); err != nil || model.Name == "" {
			return nil, fmt.Errorf("jev: invalid model entry")
		}

		models = append(models, model)
	}

	return models, nil
}
