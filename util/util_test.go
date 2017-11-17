package util

import (
	"testing"

	"reflect"
)

func TestParseRepoRef(t *testing.T) {
	examples := []struct {
		repoRef string
		iserr   bool
	}{
		{"nginx=unicorn,blueberry,ninja", false},
		{"registry.hipster.io/hype/sdn", false},
		{"mesosphere/mesos~/^1\\.[0-9]+\\.[0-9]/=unter", true},
		{"registry.hipster.io/hype/drone~/v[0-9]+$/", false},
		{"cabron/pinche=x=y", true},
		{"/", true},
		{" blue", true},
		{"repo/roar~/ss=x", true},
		{"repo/boar~/ss/=x=y", true},
		{"repo/boar~/[a-z]*/=x", false},
		{"localhost:5757/qa/library/alpine", false},
	}

	for _, e := range examples {
		_, _, _, err := ParseRepoRef(e.repoRef)

		action := "should"
		if !e.iserr {
			action = "should NOT"
		}

		iserr := err != nil
		if iserr != e.iserr {
			t.Errorf(
				"Passing repository reference '%s' %s fail (error: %v)",
				e.repoRef,
				action,
				err,
			)
		}
	}
}

func TestSeparateAssumedTagNamesAndRepo(t *testing.T) {
	examples := []struct {
		repoRef         string
		repoWithFilter  string
		assumedTagNames []string
		iserr           bool
	}{
		{"nginx=unicorn,blueberry,ninja", "nginx", []string{"unicorn", "blueberry", "ninja"}, false},
		{"registry.hipster.io/hype/sdn", "registry.hipster.io/hype/sdn", nil, false},
		{"mesos~/^1\\.[0-9]+\\.[0-9]+$/=unter", "mesos~/^1\\.[0-9]+\\.[0-9]+$/", []string{"unter"}, false},
		{"registry.hipster.io/hype/drone~/v[0-9]+$/", "registry.hipster.io/hype/drone~/v[0-9]+$/", nil, false},
		{"cabron/pinche=x=y", "", nil, true},
	}

	for _, e := range examples {
		repoWithFilter, assumedTagNames, err := SeparateAssumedTagNamesAndRepo(e.repoRef)

		if repoWithFilter != e.repoWithFilter {
			t.Errorf(
				"Unexpected repo[~/FILTER/] '%s' trimmed from '%s' (expected: '%s')",
				repoWithFilter,
				e.repoRef,
				e.repoWithFilter,
			)
		}

		if !reflect.DeepEqual(assumedTagNames, e.assumedTagNames) {
			t.Errorf(
				"Unexpected tag names '%v' trimmed from '%s' (expected: '%v')",
				assumedTagNames,
				e.repoRef,
				e.assumedTagNames,
			)
		}

		action := "should"
		if !e.iserr {
			action = "should NOT"
		}

		iserr := err != nil
		if iserr != e.iserr {
			t.Errorf(
				"Passing repository reference '%s' %s trigger an error",
				e.repoRef,
				action,
			)
		}
	}
}

func TestSeparateFilterAndRepo(t *testing.T) {
	examples := []struct {
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

	for _, e := range examples {
		repo, filter, err := SeparateFilterAndRepo(e.repoWithFilter)

		if repo != e.repo {
			t.Errorf(
				"Unexpected repository name '%s' trimmed from '%s' (expected: '%s')",
				repo,
				e.repoWithFilter,
				e.repo,
			)
		}

		if filter != e.filter {
			t.Errorf(
				"Unexpected repository filter '%s' trimmed from '%s' (expected: '%s')",
				filter,
				e.repoWithFilter,
				e.filter,
			)
		}

		action := "should"
		if !e.iserr {
			action = "should NOT"
		}

		iserr := err != nil
		if iserr != e.iserr {
			t.Errorf(
				"Passing repo[~/FILTER/] '%s' %s trigger an error",
				e.repoWithFilter,
				action,
			)
		}
	}
}

func TestDoesMatch(t *testing.T) {
	examples := []struct {
		s       string
		pattern string
		matched bool
	}{
		{"latest", "^latest$", true},
		{"v1.0.1", "^v1\\.0\\.1$", true},
		{"barbos", ".*", true},
		{"3.4", "*", false},
	}

	for _, e := range examples {
		matched := DoesMatch(e.s, e.pattern)

		action := "should"
		if !e.matched {
			action = "should not"
		}

		if matched != e.matched {
			t.Errorf(
				"String '%s' %s match pattern '%s'",
				e.s,
				action,
				e.pattern,
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
			t.Errorf(
				"Unexpected path '%s' generated from hostname '%s' (expected: '%s')",
				output,
				input,
				expected,
			)
		}
	}
}
