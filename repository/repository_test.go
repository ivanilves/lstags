package repository

import (
	"testing"

	"fmt"

	"github.com/stretchr/testify/assert"
)

func TestParseRef(t *testing.T) {
	type expectation struct {
		Registry          string
		IsDefaultRegistry bool
		Full              string
		Name              string
		Path              string
		Tags              []string
		Filter            string
		WebSchema         string
		IsCorrect         bool
	}

	var testCases = map[string]expectation{
		"alpine":                               {"registry.hub.docker.com", true, "registry.hub.docker.com/alpine", "alpine", "library/alpine", []string{}, ".*", "https://", true},
		"alp@ne":                               {"", true, "", "", "", []string{}, "", "", false},
		"localhost/bitcoin/robot":              {"localhost", false, "localhost/bitcoin/robot", "localhost/bitcoin/robot", "bitcoin/robot", []string{}, ".*", "http://", true},
		"localhost:5000/nada/mindundi":         {"localhost:5000", false, "localhost:5000/nada/mindundi", "localhost:5000/nada/mindundi", "nada/mindundi", []string{}, ".*", "http://", true},
		"localhost:7eff/nada/mindundi":         {"", true, "", "", "", []string{}, "", "", false},
		"quay.io/coreos/awscli:master":         {"quay.io", false, "quay.io/coreos/awscli", "quay.io/coreos/awscli", "coreos/awscli", []string{"master"}, "", "https://", true},
		"registry.org/some/repo=latest,stable": {"registry.org", false, "registry.org/some/repo", "registry.org/some/repo", "some/repo", []string{"latest", "stable"}, "", "https://", true},
		"registry.org/some/repo=lat!st,stable": {"", true, "", "", "", []string{}, "", "", false},
		"registry.org/some/repo~/^v1/":         {"registry.org", false, "registry.org/some/repo", "registry.org/some/repo", "some/repo", []string{}, "^v1", "https://", true},
		"registry.org/some/repo~|^v1|":         {"", true, "", "", "", []string{}, "", "", false},
		"ivanilves/lstags":                     {"registry.hub.docker.com", true, "registry.hub.docker.com/ivanilves/lstags", "ivanilves/lstags", "ivanilves/lstags", []string{}, ".*", "https://", true},
	}

	assert := assert.New(t)

	for ref, expected := range testCases {
		fmt.Printf("* ParseRef: %-40s [correct: %t]\n", ref, expected.IsCorrect)

		r, err := ParseRef(ref)

		if expected.IsCorrect {
			assert.Nil(err, "should be no error")
		} else {
			assert.NotNil(err, "should be an error")
		}

		if err != nil {
			continue
		}

		assert.Equal(
			r.Registry(), expected.Registry,
			"unexpected registr (ref: %s)",
			ref,
		)

		assert.Equal(
			r.IsDefaultRegistry(), expected.IsDefaultRegistry,
			"should be served from default registry (ref: %s)",
			ref,
		)

		assert.Equal(
			r.Full(), expected.Full,
			"unexpected full repo spec (ref: %s)",
			ref,
		)

		assert.Equal(
			r.Name(), expected.Name,
			"unexpected repo name (ref: %s)",
			ref,
		)

		assert.Equal(
			r.Path(), expected.Path,
			"unexpected repo path (ref: %s)",
			ref,
		)

		assert.Equal(
			r.Tags(), expected.Tags,
			"unexpected tag spec (ref: %s)",
			ref,
		)

		assert.Equal(
			r.Filter(), expected.Filter,
			"unexpected filter spec (ref: %s)",
			ref,
		)

		assert.Equal(
			r.WebSchema(), expected.WebSchema,
			"unexpected connection schema (ref: %s)",
			ref,
		)
	}
}
