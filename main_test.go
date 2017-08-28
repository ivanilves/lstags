package main

import (
	"testing"

	"reflect"
)

var registryTags = map[string]string{
	"v1.1":   "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac",
	"v1.2":   "sha256:9b618bebfbce63619fcd6c9e00967ffa3bf075d8d331be931012e1ba3472d4d1",
	"latest": "sha256:33fa8a96ed94cd7580c812891e7771be3a0ad510828ea76351162e5781456da2",
}

var localTags = map[string]string{
	"v1.1":   "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac",
	"v1.2.1": "sha256:3c7f921d1301bc662e18643190f9404679ee28326f2b6d68d3c721466fc3c6c2",
	"latest": "sha256:d23eba72cd72037b1106b73c6e7c11a101bc7ba09cb25f9ee7157b792c528f09",
}

func TestShortify(t *testing.T) {
	const cutToLength = 10

	shortString := "so short!"
	longString := "size does matter after all!"

	var resultString string

	resultString = shortify(shortString, cutToLength)
	if resultString != shortString {
		t.Fatalf(
			"String with length <= %d should not be modified (We got: %s => %s)",
			cutToLength,
			shortString,
			resultString,
		)
	}

	resultString = shortify(longString, cutToLength)
	if len(resultString) != cutToLength {
		t.Fatalf(
			"String with length > %d should be cut exactly to this length (We got: %s => %s, length: %d)",
			cutToLength,
			longString,
			resultString,
			len(resultString),
		)
	}
	if resultString != longString[0:cutToLength] {
		t.Fatalf(
			"Should return first %d characters of the passed string (We got: %s => %s)",
			cutToLength,
			longString,
			resultString,
		)
	}
}

func TestConcatTagNames(t *testing.T) {
	tagNames := concatTagNames(registryTags, localTags)

	expectedTagNames := []string{"latest", "v1.1", "v1.2", "v1.2.1"}

	if !reflect.DeepEqual(tagNames, expectedTagNames) {
		t.Fatalf(
			"Should merge and sort registry and local tag names (Expected: %v / Got: %v)\nregistry: %v\nlocal: %v",
			expectedTagNames,
			tagNames,
			registryTags,
			localTags,
		)
	}
}

func TestGetShortImageID(t *testing.T) {
	const imageID = "sha256:57848d7a78d09ac3991b067a6e10ad89f40fbb09c4bdf6e1029fc5141dd3f07e"
	const expectedShortImageID = "57848d7a78d0"

	shortImageID := getShortImageID(imageID)

	if shortImageID != expectedShortImageID {
		t.Fatalf(
			"Should return first %d characters of the image ID (Expected: %s / Got: %s)",
			len(expectedShortImageID),
			expectedShortImageID,
			shortImageID,
		)
	}
}

func TestFormatImageIDs(t *testing.T) {
	localImageIDs := map[string]string{
		"v1.1":   "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac",
		"latest": "sha256:33fa8a96ed94cd7580c812891e7771be3a0ad510828ea76351162e5781456da2",
	}

	tagNames := []string{"v1.0", "v1.1", "v1.2", "latest"}

	expectedImageIDs := map[string]string{
		"v1.0":   "n/a",
		"v1.1":   "7abd16433f3b",
		"v1.2":   "n/a",
		"latest": "33fa8a96ed94",
	}

	imageIDs := formatImageIDs(localImageIDs, tagNames)

	if !reflect.DeepEqual(imageIDs, expectedImageIDs) {
		t.Fatalf(
			"Should format image IDs for givent tags correctly:\n* Expected: %#v\n* Got: %#v",
			expectedImageIDs,
			imageIDs,
		)
	}
}

func TestGetDigest(t *testing.T) {
	expectedDigests := map[string]string{
		"v1.1":   "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac",
		"v1.2":   "sha256:9b618bebfbce63619fcd6c9e00967ffa3bf075d8d331be931012e1ba3472d4d1",
		"v1.2.1": "sha256:3c7f921d1301bc662e18643190f9404679ee28326f2b6d68d3c721466fc3c6c2",
		"latest": "sha256:33fa8a96ed94cd7580c812891e7771be3a0ad510828ea76351162e5781456da2",
	}

	for tag, expectedDigest := range expectedDigests {
		digest := getDigest(tag, registryTags, localTags)

		if digest != expectedDigest {
			t.Fatalf(
				"Should get correct image digest for tag %s:\n* Expected: %s\n* Got: %s",
				tag,
				expectedDigest,
				digest,
			)
		}
	}
}

func TestGetState(t *testing.T) {
	expectedStates := map[string]string{
		"v1.1":   "PRESENT",
		"v1.2":   "ABSENT",
		"v1.2.1": "LOCAL-ONLY",
		"latest": "CHANGED",
	}

	for tag, expectedState := range expectedStates {
		state := getState(tag, registryTags, localTags)

		if state != expectedState {
			t.Fatalf(
				"Should get correct image state for tag %s:\n* Expected: %s\n* Got: %s",
				tag,
				expectedState,
				state,
			)
		}
	}
}

func TestGetRepoRegistryName(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"nginx": "library/nginx",
		"registry.nerd.io/hype/cube": "hype/cube",
		"observability/metrix":       "observability/metrix",
	}

	for input, expected := range expectations {
		output := getRepoRegistryName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected registry repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoLocalNameForPublicRegistry(t *testing.T) {
	const registry = "registry.hub.docker.com"

	expectations := map[string]string{
		"library/nginx": "nginx",
		"hype/cube":     "hype/cube",
	}

	for input, expected := range expectations {
		output := getRepoLocalName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected local repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}

func TestGetRepoLocalNameForPrivateRegistry(t *testing.T) {
	const registry = "registry.nerd.io"

	expectations := map[string]string{
		"empollon/nginx":             "registry.nerd.io/empollon/nginx",
		"registry.nerd.io/hype/cube": "registry.nerd.io/hype/cube",
	}

	for input, expected := range expectations {
		output := getRepoLocalName(input, registry)

		if output != expected {
			t.Fatalf(
				"Got unexpected registry repo name: %s => %s\n* Expected: %s",
				input,
				output,
				expected,
			)
		}
	}
}
