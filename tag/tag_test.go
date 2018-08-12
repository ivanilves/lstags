package tag

import (
	"testing"

	"strconv"
	"strings"
	"time"
)

func TestNew(t *testing.T) {
	var params = map[string]string{
		"name":   "latest",
		"digest": "sha256:c92260fe6357ac1cdd79e86e23fa287701c5edd2921d243a253fd21c9f0012ae",
	}

	tg, err := New(params["name"], Options{Digest: params["digest"]})

	if err != nil {
		t.Fatalf("Unable to create new tag: %s", err.Error())
	}

	if tg.Name() != params["name"] {
		t.Fatalf("Unexpected name: '%s' (expected '%s')", tg.Name(), params["name"])
	}

	if tg.GetDigest() != params["digest"] {
		t.Fatalf("Unexpected digest: '%s' (expected '%s')", tg.GetDigest(), params["digest"])
	}

	if tg.GetShortDigest() != params["digest"][0:40] {
		t.Fatalf("Unexpected short digest: '%s' (expected '%s')", tg.GetShortDigest(), params["digest"][0:40])
	}
}

func TestNewWithShortDigest(t *testing.T) {
	var params = map[string]string{
		"name":   "latest",
		"digest": "csum:iamkindashort",
	}

	tg, err := New(params["name"], Options{Digest: params["digest"]})

	if err != nil {
		t.Fatalf("Unable to create new tag with a short digest: %s", err.Error())
	}

	if tg.GetDigest() != params["digest"] {
		t.Fatalf("Unexpected digest: '%s' (expected '%s')", tg.GetDigest(), params["digest"])
	}

	if tg.GetShortDigest() != params["digest"] {
		t.Fatalf("Unexpected short digest: '%s' (expected '%s')", tg.GetShortDigest(), params["digest"])
	}
}

func TestNew_WithEmptyName(t *testing.T) {
	_, err := New(
		"",
		Options{Digest: "sha256:c92260fe6357ac1cdd79e86e23fa287701c5edd2921d243a253fd21c9f0012ae"},
	)

	if err == nil {
		t.Fatalf("Was able to create tag with empty name")
	}
}

func TestNew_WithEmptyDigest(t *testing.T) {
	_, err := New(
		"latest",
		Options{Digest: ""},
	)

	if err == nil {
		t.Fatalf("Was able to create tag with empty image digest")
	}
}

//
// Here we generate fake tags for our tests
//

func getRemoteTags() map[string]*Tag {
	seeds := []struct {
		name   string
		digest string
	}{
		{"latest", "sha256:c92260fe6357ac1cdd79e86e23fa287701c5edd2921d243a253fd21c9f0012ae"},
		{"v1.1", "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac"},
		{"v1.2", "sha256:7f7f94f26d23f7aca80a33732161af068f9f62fbe0e824a58cf3a39d209cfa77"},
		{"v1.3.1", "sha256:9fb0e8a4f629b72a0a69aef357e637e4145b6588f04f1540a31a0d2e030ea7da"},
		{"v1.3.2", "sha256:fc41473fc36c09222a29ffce9eaf5732fae91c3fabfa40aa878f600e13c7fed3"},
	}

	tags := make(map[string]*Tag, 0)

	for _, seed := range seeds {
		tags[seed.name], _ = New(seed.name, Options{Digest: seed.digest})
	}

	return tags
}

func getLocalTags() map[string]*Tag {
	seeds := []struct {
		name    string
		digest  string
		imageID string
	}{
		{"latest", "sha256::8ffc20b5be0e391f07f270bf79441fbea3c8b67200e5316bdefad9e0ca80277b", "sha256:883e3a5b24d7b46f81436bfc85564a676aa021a2c8adedc3ac6ab12ac06fdd95"},
		{"v1.0", "sha256:fe4286e7b852dc6aad6225239ecb32691f15f20b0d4354defb4ca4957958b2f0", "sha256:c9a69a36ff3ce76d7970df83bd438f0f1bc0363a3b4707b42542ea20ba4282f4"},
		{"v1.2", "sha256:7f7f94f26d23f7aca80a33732161af068f9f62fbe0e824a58cf3a39d209cfa77", "4c4ebb9614ef823bd04e5eba65e59286a4314d3a063e2eaa221d38fc21723cea"},
		{"v1.3.1", "sha256:7264ba7450b6be1bfba9ab29f506293bb324f4764c41ff32dcc04379c1a69c91", ""},
		{"v1.3.2", "sha256:fc41473fc36c09222a29ffce9eaf5732fae91c3fabfa40aa878f600e13c7fed3", ""},
	}

	tags := make(map[string]*Tag, 0)

	for _, seed := range seeds {
		tags[seed.name], _ = New(seed.name, Options{Digest: seed.digest, ImageID: seed.imageID})
	}

	return tags
}

//
// Join() is a "heart" of the `tag` package.
// It requires much love & reliable testing!
//
func TestJoin_Length(t *testing.T) {
	const expected = 6

	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	length := len(tags)

	if length != expected {
		t.Fatalf(
			"Unexpected number of joined tags: %s (expected: %s)",
			strconv.Itoa(length),
			strconv.Itoa(expected),
		)
	}
}

func TestJoin_Digest(t *testing.T) {
	examples := map[string]string{
		"latest": "sha256:c92260fe6357ac1cdd79e86e23fa287701c5edd2921d243a253fd21c9f0012ae",
		"v1.0":   "sha256:fe4286e7b852dc6aad6225239ecb32691f15f20b0d4354defb4ca4957958b2f0",
		"v1.1":   "sha256:7abd16433f3bec5ee4c566ddbfc0e5255678498d5e7e2da8f41393bfe84bfcac",
		"v1.2":   "sha256:7f7f94f26d23f7aca80a33732161af068f9f62fbe0e824a58cf3a39d209cfa77",
	}

	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		digest := tags[name].GetDigest()

		if digest != expected {
			t.Fatalf(
				"Unexpected digest [%s]: %s (expected: %s)",
				name,
				digest,
				expected,
			)
		}
	}
}

func TestJoin_ImageID(t *testing.T) {
	examples := map[string]string{
		"latest": "883e3a5b24d7",
		"v1.0":   "c9a69a36ff3c",
		"v1.1":   "n/a",
		"v1.2":   "4c4ebb9614ef",
	}

	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		imageID := tags[name].GetImageID()

		if imageID != expected {
			t.Fatalf(
				"Unexpected image ID [%s]: %s (expected: %s)",
				name,
				imageID,
				expected,
			)
		}
	}
}

func TestJoin_State(t *testing.T) {
	examples := map[string]string{
		"latest": "CHANGED",
		"v1.0":   "LOCAL-ONLY",
		"v1.1":   "ABSENT",
		"v1.2":   "PRESENT",
		"v1.3.1": "CHANGED",
		"v1.3.2": "PRESENT",
	}

	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		state := tags[name].GetState()

		if state != expected {
			t.Fatalf(
				"Unexpected state [%s]: %s (expected: %s)",
				name,
				state,
				expected,
			)
		}
	}
}

func TestJoin_State_WithAssumedTagNames(t *testing.T) {
	assumedTagNames := []string{"v1.3.2", "v1.4.1"}

	examples := map[string]string{
		"v1.3.2": "PRESENT",
		"v1.4.1": "ASSUMED",
	}

	_, _, tags := Join(getRemoteTags(), getLocalTags(), assumedTagNames)

	for name, expected := range examples {
		state := tags[name].GetState()

		if state != expected {
			t.Fatalf(
				"Unexpected state [%s]: %s (expected: %s)",
				name,
				state,
				expected,
			)
		}
	}
}

func TestJoin_NeedsPull(t *testing.T) {
	examples := map[string]bool{
		"v1.3.1": true,
		"v1.3.2": false,
	}
	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		needsPull := tags[name].NeedsPull()

		if needsPull != expected {
			t.Fatalf(
				"Unexpected pull need [%s]: %v (expected: %v)",
				name,
				needsPull,
				expected,
			)
		}
	}
}

func TestJoin_NeedsPush(t *testing.T) {
	examples := map[string]bool{
		"v1.3.1": false,
		"v1.3.2": false,
	}
	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		needsPush := tags[name].NeedsPush(false)

		if needsPush != expected {
			t.Fatalf(
				"Unexpected push need [%s]: %v (expected: %v)",
				name,
				needsPush,
				expected,
			)
		}
	}
}

func TestJoin_NeedsPush_WithPushUpdate(t *testing.T) {
	examples := map[string]bool{
		"v1.3.1": true,
		"v1.3.2": false,
	}
	_, _, tags := Join(getRemoteTags(), getLocalTags(), nil)

	for name, expected := range examples {
		needsPush := tags[name].NeedsPush(true)

		if needsPush != expected {
			t.Fatalf(
				"Unexpected push need [%s]: %v (expected: %v)",
				name,
				needsPush,
				expected,
			)
		}
	}
}

func TestCollect(t *testing.T) {
	keys, tagNames, tagMap := Join(getRemoteTags(), getLocalTags(), nil)

	tags := Collect(keys, tagNames, tagMap)

	if len(tags) != len(tagMap) {
		t.Fatalf(
			"number of tags is not equal to one of original map (%d vs %d)\n%+v\nvs\n%+v",
			len(tags),
			len(tagMap),
			tags,
			tagMap,
		)
	}

	for _, tg := range tags {
		_, defined := tagMap[tg.Name()]

		if !defined {
			t.Fatalf(
				"tag '%s' (from %+v) not present in original map: %+v",
				tg.Name(),
				tags,
				tagMap,
			)
		}
	}
}

func TestCutImageID(t *testing.T) {
	testCases := map[string]string{
		"sha256:249aca4f9e076c53d9fa7cb591cbc0d013f54da93c393f054f5d70c8705c8e6c": "249aca4f9e07",
		"5bef08742407efd622d243692b79ba0055383bbce12900324f75e56f589aedb0":        "5bef08742407",
		"sha256:031e148a88a3":                                                     "031e148a88a3",
		"131e158a88a3":                                                            "131e158a88a3",
		"csum:something":                                                          "something",
		"948995":                                                                  "948995",
	}

	for passed, expected := range testCases {
		imageID := cutImageID(passed)

		if imageID != expected {
			t.Fatalf(
				"Unexpected image ID: %s (passed: %s / expected: %s)",
				imageID,
				passed,
				expected,
			)
		}
	}
}

func TestCreated(t *testing.T) {
	expectedTimestamp := time.Now().Unix()

	tg, _ := New("latest", Options{Digest: "csum:something", Created: expectedTimestamp})

	timestamp := tg.GetCreated()
	if timestamp != expectedTimestamp {
		t.Fatalf(
			"unexpected creation timestamp: %d (expected: %d)",
			timestamp,
			expectedTimestamp,
		)
	}

	expectedTime := time.Unix(timestamp, 0)
	expectedTimeString := strings.Split(expectedTime.Format(time.RFC3339), "+")[0]
	timeString := tg.GetCreatedString()
	if timeString != expectedTimeString {
		t.Fatalf(
			"unexpected string form of creation time: %s (expected: %s)",
			timeString,
			expectedTimeString,
		)
	}
}
