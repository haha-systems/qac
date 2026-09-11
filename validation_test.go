package qac_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/haha-systems/qac"
)

func validRequest() qac.Request {
	return qac.Request{
		Context:   qac.Context{CurrentResource: "a"},
		Resources: []qac.Resource{{ID: "a", Available: true}},
	}
}

func TestValidateRequestRejectsEmptyInput(t *testing.T) {
	if err := qac.ValidateRequest(qac.Request{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsEmptyResourceID(t *testing.T) {
	req := validRequest()
	req.Resources[0].ID = ""
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsDuplicateResourceIDs(t *testing.T) {
	err := qac.ValidateRequest(qac.Request{Resources: []qac.Resource{{ID: "a"}, {ID: "a"}}})
	if err == nil || !strings.Contains(err.Error(), "duplicate resource") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRequestRejectsInvalidResourceValue(t *testing.T) {
	req := validRequest()
	req.Resources[0].Cost = 1.1
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsInvalidSignal(t *testing.T) {
	req := validRequest()
	req.Context.Uncertainty = math.NaN()
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsNegativeFailures(t *testing.T) {
	req := validRequest()
	req.Context.FailedAttempts = -1
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsUnknownCurrentResource(t *testing.T) {
	req := validRequest()
	req.Context.CurrentResource = "missing"
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestRejectsBudgetForUnknownResource(t *testing.T) {
	req := validRequest()
	req.Budget.Resources = map[string]qac.ResourceBudget{"missing": {}}
	if err := qac.ValidateRequest(req); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequestDoesNotMutateRequest(t *testing.T) {
	remaining := 2
	req := validRequest()
	req.Budget.Resources = map[string]qac.ResourceBudget{"a": {Enabled: true, RemainingInvocations: &remaining}}
	before := req
	if err := qac.ValidateRequest(req); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(req, before) {
		t.Fatalf("request was mutated: %#v", req)
	}
}
