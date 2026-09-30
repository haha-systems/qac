package jev

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ValidationIssue is a server-side request error. Its input value is excluded
// because it can contain application state.
type ValidationIssue struct {
	Location []any
	Message  string
	Type     string
}

// APIError reports an unsuccessful HTTP status and safe validation details.
type APIError struct {
	StatusCode int
	Details    []ValidationIssue
}

func (e *APIError) Error() string {
	return fmt.Sprintf("jev: HTTP status %d", e.StatusCode)
}

func apiError(response *http.Response, data []byte) *APIError {
	err := &APIError{StatusCode: response.StatusCode}
	if response.StatusCode != http.StatusUnprocessableEntity || len(data) == 0 {
		return err
	}

	var body struct {
		Detail []struct {
			Location []any  `json:"loc"`
			Message  string `json:"msg"`
			Type     string `json:"type"`
		} `json:"detail"`
	}
	if json.Unmarshal(data, &body) != nil {
		return err
	}

	for _, issue := range body.Detail {
		if len(issue.Message) > 256 {
			issue.Message = issue.Message[:256]
		}
		if len(issue.Location) > 16 {
			issue.Location = issue.Location[:16]
		}
		err.Details = append(err.Details, ValidationIssue{
			Location: issue.Location, Message: issue.Message, Type: issue.Type,
		})
	}

	return err
}

func readResponse(response *http.Response) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("jev: cannot read HTTP response: %w", err)
	}

	return data, nil
}
