package collection

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
)

func makeTags() []*tag.Tag {
	var seed = map[string]string{
		"latest": "sha256:6905a419c4fe7e29acb03cabd2aa9a01226c69277bf718faff52537b1b7b38ab",
		"v1.0.1": "sha256:5dac334cf325e506d4304baaf990f2b52e7e4f5c21388e3f1bcad89b9aa830c8",
	}

	var tags = make([]*tag.Tag, 0)

	for name, digest := range seed {
		tg, _ := tag.New(name, tag.Options{Digest: digest})

		tags = append(tags, tg)
	}

	return tags
}

func makeRefTags(refs ...string) map[string][]*tag.Tag {
	var refTags = make(map[string][]*tag.Tag)

	for _, ref := range refs {
		refTags[ref] = makeTags()
	}

	return refTags
}

func TestRefHasTags(t *testing.T) {
	var testCases = []struct {
		ref     string
		refTags map[string][]*tag.Tag
		hasTags bool
	}{
		{"ninja.turtle/rafael", makeRefTags("alpine", "ninja.turtle/rafael"), true},
		{"ninja.turtle/rafael", makeRefTags(), false},
		{"ninja.turtle/leonardo", makeRefTags("alpine", "ninja.turtle/rafael", "ninja.turtle/donatello"), false},
		{"ninja.turtle/donatello", makeRefTags("alpine", "ninja.turtle/rafael", "ninja.turtle/donatello"), true},
	}

	assert := assert.New(t)

	for _, tc := range testCases {
		actual := refHasTags(tc.ref, tc.refTags)

		action := "should"
		if !tc.hasTags {
			action = "should NOT"
		}

		assert.Equal(
			tc.hasTags,
			actual,
			"Reference '%s' %s have tags assigned:\n%v",
			tc.ref,
			action,
			tc.refTags,
		)
	}
}

func TestContains(t *testing.T) {
	assert := assert.New(t)

	assert.True(
		contains([]string{"ninja", "alpine", "busybox"}, "alpine"),
	)

	assert.False(
		contains([]string{"ninja", "busybox"}, "alpine"),
	)
}

func TestNew(t *testing.T) {
	var testCases = []struct {
		refs      []string
		refTags   map[string][]*tag.Tag
		isCorrect bool
	}{
		{
			[]string{"alpine:3.7", "busybox:latest", "quay.io/calico/ctl"},
			makeRefTags("alpine:3.7", "busybox:latest", "quay.io/calico/ctl"),
			true,
		},
		{
			[]string{"alpine:3.7", "busybox:latest", "quay.io/calico/ctl"},
			makeRefTags("alpine:3.7", "busybox:stable", "quay.io/calico/ctl"),
			false,
		},
		{
			[]string{"quay.io/coreos/flannel", "golang"},
			makeRefTags("quay.io/coreos/flannel", "nginx"),
			false,
		},
		{
			[]string{"containers.hype.org/kubernetes/kube-proxy:latest", "quay.io/calico/node:0.9"},
			makeRefTags("containers.hype.org/kubernetes/kube-proxy:latest", "quay.io/calico/node:0.9", "openhype/hipstermind:stable"),
			false,
		},
		{
			[]string{},
			makeRefTags("openresty/openresty", "gcr.io/google_containers/pause-amd64"),
			false,
		},
		{
			[]string{"openhype/openhype~/v1/", "gcr.io/google_containers/pause-amd64=3.0,3.1"},
			makeRefTags("openhype/openhype~/v1/", "gcr.io/google_containers/pause-amd64=3.0,3.1"),
			true,
		},
		{
			[]string{"openhype/openhype~~~/!v1/", "gcr.io/google_containers/pause-amd64=3.0,3.1"},
			makeRefTags("openhype/openhype~~~/!v1/", "gcr.io/google_containers/pause-amd64=3.0,3.1"),
			false,
		},
		{
			[]string{"openhype/openhype~/v1/", "gcr.io/google_containers/pause-amd64::3.0,3.1"},
			makeRefTags("openhype/openhype~/v1/", "gcr.io/google_containers/pause-amd64::3.0,3.1"),
			false,
		},
	}

	assert := assert.New(t)

	for _, tc := range testCases {
		cn, err := New(tc.refs, tc.refTags)

		if tc.isCorrect {
			assert.NotNil(cn)
			assert.Nil(err)
		} else {
			assert.Nil(cn)
			assert.NotNil(err)
		}

		if err != nil {
			continue
		}
	}
}

func TestRefs(t *testing.T) {
	var refs = []string{"alpine", "busybox"}

	cn, _ := New(refs, makeRefTags(refs...))

	assert.Equal(t, refs, cn.Refs())
}

func TestRepos(t *testing.T) {
	var refs = []string{"alpine", "busybox"}
	var repos, _ = repository.ParseRefs(refs)

	cn, _ := New(refs, makeRefTags(refs...))

	assert.Equal(t, repos, cn.Repos())
}

func TestRepo(t *testing.T) {
	var refs = []string{"alpine", "busybox"}
	var repo, _ = repository.ParseRef("busybox")

	cn, _ := New(refs, makeRefTags(refs...))

	assert.Equal(t, repo, cn.Repo("busybox"))
	assert.Nil(t, cn.Repo("idonotexistanyway"))
}

func TestTags(t *testing.T) {
	var refs = []string{"nginx:stable", "debian:latest"}
	var tags = makeTags()

	var refTags = make(map[string][]*tag.Tag)

	for _, ref := range refs {
		refTags[ref] = tags
	}

	cn, _ := New(refs, refTags)

	assert.Equal(t, tags, cn.Tags("nginx:stable"))
	assert.Nil(t, cn.Tags("idonotexist"))
}

func TestTagMap(t *testing.T) {
	var refs = []string{"nginx:stable", "debian:latest"}

	var refTags = makeRefTags(refs...)

	var tagRefMap = make(map[string]map[string]*tag.Tag)

	for ref, tags := range refTags {
		tagMap := make(map[string]*tag.Tag)

		for _, tg := range tags {
			tagMap[tg.Name()] = tg
		}
		tagRefMap[ref] = tagMap
	}

	cn, _ := New(refs, refTags)

	for _, ref := range refs {
		assert.Equal(t, tagRefMap[ref], cn.TagMap(ref))
	}

	assert.Nil(t, cn.TagMap("wellidonotexist"))
}

func TestRepoCount(t *testing.T) {
	var refs = []string{"nginx:stable", "debian:latest"}

	cn, _ := New(refs, makeRefTags(refs...))

	assert.Equal(t, len(refs), cn.RepoCount())
}

func TestTagCount(t *testing.T) {
	var refs = []string{"nginx:stable", "debian:latest"}
	var refTags = makeRefTags(refs...)

	var tagCount = 0

	for _, tags := range refTags {
		tagCount += len(tags)
	}

	cn, _ := New(refs, refTags)

	assert.Equal(t, tagCount, cn.TagCount())
}

func TestTaggedRefs(t *testing.T) {
	var refs = []string{"nginx", "debian"}
	var refTags = makeRefTags(refs...)

	var taggedRefs = make([]string, 0)

	for _, ref := range refs {
		repo, _ := repository.ParseRef(ref)

		tags := refTags[ref]

		for _, tg := range tags {

			taggedRef := repo.Name() + ":" + tg.Name()

			taggedRefs = append(taggedRefs, taggedRef)
		}
	}

	cn, _ := New(refs, refTags)

	assert.Equal(t, taggedRefs, cn.TaggedRefs())
}
