package jev_test

import (
	"context"
	"os"
	"testing"

	"github.com/haha-systems/qac/jev"
)

func TestLiveAPI(t *testing.T) {
	if os.Getenv("JEV_LIVE_TEST") != "1" {
		t.Skip("set JEV_LIVE_TEST=1 to call the TypeSafe API")
	}

	client, err := jev.New(jev.Config{APIKey: os.Getenv("TYPESAFE_API_KEY")})
	if err != nil {
		t.Fatal(err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil || len(models) == 0 {
		t.Fatalf("list models returned %d models: %v", len(models), err)
	}

	response, err := client.SystemOne(context.Background(), jev.Request{
		State: "A short hello.",
		Questions: map[string]jev.Question{
			"kind": jev.ChoiceQuestion{
				Instructions: "Which kind of message is this?",
				Criteria:     map[string]any{"greeting": "A greeting", "warning": "A warning"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if answer, ok := response.Answers["kind"].(jev.ChoiceAnswer); !ok || answer.Choice != "greeting" {
		t.Fatalf("unexpected live answer: %#v", response.Answers["kind"])
	}
}
