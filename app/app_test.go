package app

import (
	"testing"
)

func TestSeparateFilterAndRepo(t *testing.T) {
	expected := []struct {
		repoWithFilter string
		repo           string
		filter         string
		iserr          bool
	}{
		{"nginx", "nginx", ".*", false},
		{"registry.hipster.io/hype/sdn", "registry.hipster.io/hype/sdn", ".*", false},
		{"mesosphere/mesos~/^1\\.[0-9]+\\.[0-9]+$/", "mesosphere/mesos", "^1\\.[0-9]+\\.[0-9]+$", false},
		{"registry.hipster.io/hype/drone~/v[0-9]+$/", "registry.hipster.io/hype/drone", "v[0-9]+$", false},
		{"bogohost:5000/hype/drone~/v[0-9]+$/", "bogohost:5000/hype/drone", "v[0-9]+$", false},
		{"registry.clown.bad/cache/merd~x[0-9]", "", "", true},
		{"cabron/~plla~x~", "", "", true},
	}

	for _, e := range expected {
		repo, filter, err := SeparateFilterAndRepo(e.repoWithFilter)

		if repo != e.repo {
			t.Fatalf(
				"Unexpected repository name '%s' trimmed from '%s' (expected: '%s')",
				repo,
				e.repoWithFilter,
				e.repo,
			)
		}

		if filter != e.filter {
			t.Fatalf(
				"Unexpected repository filter '%s' trimmed from '%s' (expected: '%s')",
				filter,
				e.repoWithFilter,
				e.filter,
			)
		}

		iserr := err != nil
		if iserr != e.iserr {
			t.Fatalf("Passing badly formatted repository '%s' should trigger an error", e.repoWithFilter)
		}
	}
}

func TestDoesMatch(t *testing.T) {
	expected := []struct {
		s       string
		pattern string
		matched bool
	}{
		{"latest", "^latest$", true},
		{"v1.0.1", "^v1\\.0\\.1$", true},
		{"barbos", ".*", true},
		{"3.4", "*", false},
	}

	for _, e := range expected {
		matched := DoesMatch(e.s, e.pattern)

		action := "should"
		if !e.matched {
			action = "should not"
		}

		if matched != e.matched {
			t.Fatalf(
				"String '%s' %s match pattern '%s'",
				e.s,
				action,
				e.pattern,
			)
		}
	}
}

func TestGetRegistryNameFromRepo(t *testing.T) {
	expected := map[string]string{
		"mesosphere/marathon":             dockerHub,
		"bogohost/my/inner/troll":         dockerHub,
		"registry.hipsta.io/hype/hotshit": "registry.hipsta.io",
		"localhost/my/image":              "localhost",
		"bogohost:5000/mymymy/img":        "bogohost:5000",
	}

	for repo, expectedRegistryName := range expected {
		registryName := GetRegistryNameFromRepo(repo, dockerHub)

		if registryName != expectedRegistryName {
			t.Fatalf(
				"Got unexpected Docker registry name '%s' from repo '%s' (expected: '%s')",
				registryName,
				repo,
				expectedRegistryName,
			)
		}
	}
}
func TestGeneratePathFromHostname(t *testing.T) {
	examples := map[string]string{
		"localhost":               "/localhost",
		"localhost:5000":          "/localhost",
		"registry.company.com":    "/registry/company/com",
		"dockerz.hipster.io:8443": "/dockerz/hipster/io",
	}

	for input, expected := range examples {
		output := GeneratePathFromHostname(input)

		if output != expected {
			t.Fatalf(
				"Unexpected path '%s' generated from hostname '%s' (expected: '%s')",
				output,
				input,
				expected,
			)
		}
	}
}
