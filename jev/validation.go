package jev

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
)

func object(raw json.RawMessage, required ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("jev: expected JSON object")
	}

	for _, name := range required {
		value, found := fields[name]
		if !found || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("jev: missing required field %s", name)
		}
	}

	return fields, nil
}

func description(raw json.RawMessage, allowNull bool) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || (allowNull && bytes.Equal(raw, []byte("null"))) {
		return nil
	}

	switch raw[0] {
	case '"', '{', '[':
		return nil
	default:
		return fmt.Errorf("jev: expected string, object, or array")
	}
}

func validateRequest(data []byte) (map[string]json.RawMessage, error) {
	fields, err := object(data, "state", "model", "questions")
	if err != nil {
		return nil, err
	}

	if err := description(fields["state"], false); err != nil {
		return nil, fmt.Errorf("jev: invalid state shape")
	}

	questions, err := object(fields["questions"])
	if err != nil || len(questions) == 0 {
		return nil, fmt.Errorf("jev: request must contain questions")
	}

	for _, raw := range questions {
		if err := validateQuestion(raw); err != nil {
			return nil, err
		}
	}

	return questions, nil
}

func validateQuestion(raw json.RawMessage) error {
	fields, err := object(raw, "type")
	if err != nil {
		return err
	}

	if err := description(fields["instructions"], true); err != nil {
		return fmt.Errorf("jev: invalid question instructions")
	}

	var kind string
	if err := json.Unmarshal(fields["type"], &kind); err != nil {
		return fmt.Errorf("jev: invalid question type")
	}

	switch kind {
	case "choice":
		criteria, err := object(fields["criteria"])
		if err != nil {
			return fmt.Errorf("jev: invalid choice criteria")
		}

		for _, value := range criteria {
			if err := description(value, true); err != nil {
				return fmt.Errorf("jev: invalid choice description")
			}
		}
	case "score":
		var levels []json.RawMessage
		if err := json.Unmarshal(fields["criteria"], &levels); err != nil || len(levels) == 0 {
			return fmt.Errorf("jev: score requires at least one level")
		}

		for _, value := range levels {
			if err := description(value, false); err != nil {
				return fmt.Errorf("jev: invalid score level")
			}
		}
	case "noul":
		criteria := bytes.TrimSpace(fields["criteria"])
		if len(criteria) == 0 || bytes.Equal(criteria, []byte("null")) {
			return nil
		}

		values, err := object(criteria)
		if err != nil {
			return fmt.Errorf("jev: invalid yes/no criteria")
		}

		for _, key := range []string{"true", "false"} {
			if err := description(values[key], true); err != nil {
				return fmt.Errorf("jev: invalid yes/no description")
			}
		}
	default:
		return fmt.Errorf("jev: unknown question type")
	}

	return nil
}

func normalized(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func validateProbabilities(raw json.RawMessage, expected map[string]json.RawMessage) error {
	values, err := object(raw)
	if err != nil || len(values) != len(expected) {
		return fmt.Errorf("jev: invalid probability keys")
	}

	for key := range expected {
		value, found := values[key]
		var probability *float64
		if !found || json.Unmarshal(value, &probability) != nil || probability == nil || !normalized(*probability) {
			return fmt.Errorf("jev: invalid probability")
		}
	}

	return nil
}
