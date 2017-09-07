package tag

import (
	"testing"
)

var params = map[string]string{
	"name":   "latest",
	"digest": "sha256:c92260fe6357ac1cdd79e86e23fa287701c5edd2921d243a253fd21c9f0012ae",
}

func TestNewTag(t *testing.T) {
	i, err := NewTag(
		params["name"],
		params["digest"],
	)

	if err != nil {
		t.Fatalf("Unable to create new valid tag instance: %s", err.Error())
	}

	if i.GetName() != params["name"] {
		t.Fatalf("Unexpected name: '%s' (expected '%s')", i.GetName(), params["name"])
	}
	if i.GetDigest() != params["digest"] {
		t.Fatalf("Unexpected name: '%s' (expected '%s')", i.GetDigest(), params["digest"])
	}
}

func TestNewTagWithEmptyName(t *testing.T) {
	_, err := NewTag(
		"",
		params["digest"],
	)

	if err == nil {
		t.Fatalf("Was able to create tag with empty name")
	}
}

func TestNewTagWithEmptyDigest(t *testing.T) {
	_, err := NewTag(
		params["name"],
		"",
	)

	if err == nil {
		t.Fatalf("Was able to create tag with empty image digest")
	}
}
