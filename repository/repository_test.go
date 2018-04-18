package repository

import (
	"testing"

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
		IsSingle          bool
		IsCorrect         bool
	}

	var testCases = map[string]expectation{
		"alpine":                               {"registry.hub.docker.com", true, "registry.hub.docker.com/alpine", "alpine", "library/alpine", []string{}, ".*", "https://", false, true},
		"alp@ne":                               {"", true, "", "", "", []string{}, "", "", false, false},
		"localhost/bitcoin/robot":              {"localhost", false, "localhost/bitcoin/robot", "localhost/bitcoin/robot", "bitcoin/robot", []string{}, ".*", "http://", false, true},
		"localhost:5000/nada/mindundi":         {"localhost:5000", false, "localhost:5000/nada/mindundi", "localhost:5000/nada/mindundi", "nada/mindundi", []string{}, ".*", "http://", false, true},
		"localhost:7eff/nada/mindundi":         {"", true, "", "", "", []string{}, "", "", false, false},
		"quay.io/coreos/awscli:master":         {"quay.io", false, "quay.io/coreos/awscli", "quay.io/coreos/awscli", "coreos/awscli", []string{"master"}, "", "https://", true, true},
		"registry.org/some/repo=latest,stable": {"registry.org", false, "registry.org/some/repo", "registry.org/some/repo", "some/repo", []string{"latest", "stable"}, "", "https://", false, true},
		"registry.org/some/repo=lat!st,stable": {"", true, "", "", "", []string{}, "", "", false, false},
		"registry.org/some/repo~/^v1/":         {"registry.org", false, "registry.org/some/repo", "registry.org/some/repo", "some/repo", []string{}, "^v1", "https://", false, true},
		"registry.org/some/repo~|^v1|":         {"", true, "", "", "", []string{}, "", "", false, false},
		"ivanilves/lstags":                     {"registry.hub.docker.com", true, "registry.hub.docker.com/ivanilves/lstags", "ivanilves/lstags", "ivanilves/lstags", []string{}, ".*", "https://", false, true},
	}

	assert := assert.New(t)

	for ref, expected := range testCases {
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
			r.Ref(), ref,
			"passed reference should be equal to a parsed one",
		)

		assert.Equal(
			r.Registry(), expected.Registry,
			"unexpected registry (ref: %s)",
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

		if expected.IsSingle {
			assert.True(
				r.IsSingle(),
				"should be a 'single' IMAGE:TAG type of ref: %s",
				ref,
			)
		} else {
			assert.False(
				r.IsSingle(),
				"should NOT be a 'single' IMAGE:TAG type of ref: %s",
				ref,
			)
		}
	}
}

func TestGetRegistry(t *testing.T) {
	testCases := map[string]string{
		"alpine":                                  "registry.hub.docker.com",
		"alpine:3.7":                              "registry.hub.docker.com",
		"localhost:5000/nginx":                    "localhost:5000",
		"registry.company.com/secutiry/pentest":   "registry.company.com",
		"dockerz.hipster.io:8443/hype/kubernetes": "dockerz.hipster.io:8443",
	}

	assert := assert.New(t)

	for ref, expected := range testCases {
		repo, _ := ParseRef(ref)

		assert.Equal(repo.Registry(), expected)
	}
}

func TestRepositoryMatchTag(t *testing.T) {
	var repositories = []string{
		"alpine",
		"company.registry.com/corp/minilinux",
		"hipster.registry.io:8443/hype/kubernetes/webhook",
		"localhost:5000/chiringuito",
	}

	type expectation struct {
		TagsMatched    []string
		TagsNotMatched []string
	}

	var tagSpecs = map[string]expectation{
		``:             {[]string{"3.5", "3.6", "3.7", "latest"}, []string{}},
		`:3.7`:         {[]string{"3.7"}, []string{"3.5", "3.6", "latest"}},
		`=3.6,3.7`:     {[]string{"3.6", "3.7"}, []string{"3.5", "latest"}},
		`~/^latest$/`:  {[]string{"latest"}, []string{"3.5", "3.6", "3.7"}},
		`~/^3\.[57]$/`: {[]string{"3.5", "3.7"}, []string{"3.6", "latest"}},
	}

	var testCases = map[string]expectation{}

	// unite previously created structures to populate a complete test case table
	for _, r := range repositories {
		for ts, expected := range tagSpecs {
			ref := r + ts

			testCases[ref] = expected
		}
	}

	assert := assert.New(t)

	for ref, expected := range testCases {
		repo, _ := ParseRef(ref)

		for _, tag := range expected.TagsMatched {
			assert.True(repo.MatchTag(tag), "repository reference '%s' should match tag: %s", ref, tag)
		}

		for _, tag := range expected.TagsNotMatched {
			assert.False(repo.MatchTag(tag), "repository reference '%s' should NOT match tag: %s", ref, tag)
		}
	}
}

func TestRepositoryPushPrefix(t *testing.T) {
	testCases := map[string]string{
		"alpine":                                  "/registry/hub/docker/com/",
		"localhost:5000/nginx":                    "/localhost/",
		"registry.company.com/secutiry/pentest":   "/registry/company/com/",
		"dockerz.hipster.io:8443/hype/kubernetes": "/dockerz/hipster/io/",
	}

	assert := assert.New(t)

	for ref, expected := range testCases {
		repo, _ := ParseRef(ref)

		assert.Equal(repo.PushPrefix(), expected)
	}
}

func TestParseRefs(t *testing.T) {
	testCases := []struct {
		refs      []string
		isCorrect bool
	}{
		{[]string{"alpine", "busybox=stable,latest", "quay.io/coreos/hyperkube~/.*/", "gcr.io/google_containers/pause-amd64:3.0"}, true},
		{[]string{"alpine", "mindundi!lol", "quay.io/coreos/hyperkube~/.*/", "gcr.io/google_containers/pause-amd64:3.0"}, false},
	}

	assert := assert.New(t)

	for _, expected := range testCases {
		repos, err := ParseRefs(expected.refs) // passed and expected references are the same in this case

		if expected.isCorrect {
			assert.Nil(err, "should be no error")
		} else {
			assert.NotNil(err, "should be an error")
		}

		if err != nil {
			continue
		}

		refs := make([]string, len(expected.refs))

		for i, repo := range repos {
			refs[i] = repo.Ref()
		}

		assert.Equal(refs, expected.refs, "passed references should be the same as parsed ones")
	}
}
