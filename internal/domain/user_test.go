package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUserJSONFullName(t *testing.T) {
	data, err := json.Marshal(User{Username: "ivan.p", FirstName: "Иван", LastName: "Петров"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"full_name":"Иван Петров"`) {
		t.Fatalf("unexpected json: %s", data)
	}
}

func TestUserDisplayName(t *testing.T) {
	if got := (User{Username: "ivan.p"}).DisplayName(); got != "ivan.p" {
		t.Fatalf("expected username fallback, got %q", got)
	}
	if got := (User{Username: "ivan.p", FirstName: "Иван", LastName: "Петров"}).DisplayName(); got != "Иван Петров" {
		t.Fatalf("expected full name, got %q", got)
	}
}
