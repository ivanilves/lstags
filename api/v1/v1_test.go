package v1

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"testing"

	"github.com/stretchr/testify/assert"

	registrycontainer "github.com/ivanilves/lstags/api/v1/registry/container"
	"github.com/ivanilves/lstags/repository"
)

func runEnd2EndJob(pullRefs, seedRefs []string) ([]string, error) {
	apiConfig := Config{}

	api, err := New(apiConfig)
	if err != nil {
		return nil, err
	}

	collection, err := api.CollectTags(pullRefs...)
	if err != nil {
		return nil, err
	}

	if err := api.PullTags(collection); err != nil {
		return nil, err
	}

	registryContainer, err := registrycontainer.Launch()
	if err != nil {
		return nil, err
	}

	defer registryContainer.Destroy()

	if len(seedRefs) > 0 {
		if _, err := registryContainer.SeedWithImages(seedRefs...); err != nil {
			return nil, err
		}
	}

	pushConfig := PushConfig{Registry: registryContainer.Hostname()}

	pushCollection, err := api.CollectPushTags(collection, pushConfig)
	if err != nil {
		return nil, err
	}

	if err := api.PushTags(pushCollection, pushConfig); err != nil {
		return nil, err
	}

	return pushCollection.TaggedRefs(), nil
}

func TestEnd2End(t *testing.T) {
	var testCases = []struct {
		pullRefs         []string
		seedRefs         []string
		expectedPushRefs []string
		isCorrect        bool
	}{
		{
			[]string{},
			[]string{"alpine:3.7", "busybox:latest"},
			[]string{},
			false,
		},
		{
			[]string{"alpine:3.7", "busybox:latest"},
			[]string{"alpine:3.7", "busybox:latest"},
			[]string{},
			true,
		},
		{
			[]string{"alpine:3.7", "busybox:latest"},
			[]string{"alpine:3.7", "quay.io/calico/ctl:v1.6.1"},
			[]string{"busybox:latest"},
			true,
		},
		{
			[]string{"alpine:3.7", "busybox:latest", "gcr.io/google_containers/pause-amd64:3.0"},
			[]string{"alpine:3.7"},
			[]string{"busybox:latest", "gcr.io/google_containers/pause-amd64:3.0"},
			true,
		},
		{
			[]string{"idonotexist:latest", "busybox:latest"},
			[]string{},
			[]string{},
			false,
		},
		{
			[]string{"busybox:latest"},
			[]string{"idonotexist:latest"},
			[]string{},
			false,
		},
		{
			[]string{"busybox:latest", "!@#$%^&*"},
			[]string{},
			[]string{},
			false,
		},
		{
			[]string{"alpine:3.7", "busybox:latest"},
			[]string{"!@#$%^&*", "alpine:3.7"},
			[]string{},
			false,
		},
	}

	assert := assert.New(t)

	for _, testCase := range testCases {
		pushRefs, err := runEnd2EndJob(testCase.pullRefs, testCase.seedRefs)

		if testCase.isCorrect {
			assert.Nil(err, "should be no error")
		} else {
			assert.NotNil(err, "should be an error")
		}

		if err != nil {
			continue
		}

		assert.Equal(testCase.expectedPushRefs, pushRefs, fmt.Sprintf("%+v", testCase))
	}
}

func TestNew_VerboseLogging(t *testing.T) {
	assert := assert.New(t)

	New(Config{VerboseLogging: true})

	assert.Equal(log.DebugLevel, log.GetLevel())
}

func TestNew_InsecureRegistryEx(t *testing.T) {
	const ex = ".*"

	assert := assert.New(t)

	New(Config{InsecureRegistryEx: ex})

	assert.Equal(ex, repository.InsecureRegistryEx)
}

func TestNew_InvalidDockerJSONConfigFile(t *testing.T) {
	assert := assert.New(t)

	api, err := New(Config{DockerJSONConfigFile: "/i/do/not/exist/sorry"})

	assert.Nil(api)

	assert.NotNil(err)
}

func TestGetPushPrefix(t *testing.T) {
	var testCases = map[string]struct {
		prefix        string
		defaultPrefix string
	}{
		"/quay/io/":         {"", "/quay/io/"},
		"/":                 {"/", "whatever"},
		"/maco/":            {"/maco/", ""},
		"/suau/":            {"suau", ""},
		"/avegades/perdut/": {"/avegades/perdut", ""},
		"/mai/fotut/":       {"mai/fotut/", ""},
		"/entremaliat/":     {"entremaliat", "whatever"},
	}

	var assert = assert.New(t)

	for expected, input := range testCases {
		actual := getPushPrefix(input.prefix, input.defaultPrefix)

		assert.Equal(expected, actual)
	}
}

func TestGetBatchedSlices(t *testing.T) {
	var unbatched = []string{
		"unbatched/repo01",
		"unbatched/repo02",
		"unbatched/repo03",
		"unbatched/repo04",
		"unbatched/repo05",
		"unbatched/repo06",
		"unbatched/repo07",
		"unbatched/repo08",
		"unbatched/repo09",
		"unbatched/repo10",
	}

	var testCases = map[int][][]string{
		1:   [][]string{{"unbatched/repo01"}, {"unbatched/repo02"}, {"unbatched/repo03"}, {"unbatched/repo04"}, {"unbatched/repo05"}, {"unbatched/repo06"}, {"unbatched/repo07"}, {"unbatched/repo08"}, {"unbatched/repo09"}, {"unbatched/repo10"}},
		3:   [][]string{{"unbatched/repo01", "unbatched/repo02", "unbatched/repo03"}, {"unbatched/repo04", "unbatched/repo05", "unbatched/repo06"}, {"unbatched/repo07", "unbatched/repo08", "unbatched/repo09"}, {"unbatched/repo10"}},
		7:   [][]string{{"unbatched/repo01", "unbatched/repo02", "unbatched/repo03", "unbatched/repo04", "unbatched/repo05", "unbatched/repo06", "unbatched/repo07"}, {"unbatched/repo08", "unbatched/repo09", "unbatched/repo10"}},
		10:  [][]string{{"unbatched/repo01", "unbatched/repo02", "unbatched/repo03", "unbatched/repo04", "unbatched/repo05", "unbatched/repo06", "unbatched/repo07", "unbatched/repo08", "unbatched/repo09", "unbatched/repo10"}},
		11:  [][]string{{"unbatched/repo01", "unbatched/repo02", "unbatched/repo03", "unbatched/repo04", "unbatched/repo05", "unbatched/repo06", "unbatched/repo07", "unbatched/repo08", "unbatched/repo09", "unbatched/repo10"}},
		100: [][]string{{"unbatched/repo01", "unbatched/repo02", "unbatched/repo03", "unbatched/repo04", "unbatched/repo05", "unbatched/repo06", "unbatched/repo07", "unbatched/repo08", "unbatched/repo09", "unbatched/repo10"}},
	}

	var assert = assert.New(t)

	for batchSize, expectedBatchedSlices := range testCases {
		actualBatchedSlices := getBatchedSlices(batchSize, unbatched...)

		assert.Equalf(
			expectedBatchedSlices,
			actualBatchedSlices,
			"unexpected result for batch size: %d",
			batchSize,
		)
	}
}
