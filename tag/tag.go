// Package tag provides Tag abstraction to handle Docker tags (images)
// and their differences between remote registries and Docker daemon,
// i.e. what tags ara available in remote Docker registry, do we have them pulled
// in our local system, or do we have the same tags in our own local registry etc.
package tag

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tag aggregates tag-related information: tag name, image digest etc
type Tag struct {
	name    string
	digest  string
	imageID string
	created int64
	state   string
}

// Options holds optional parameters for Tag creation
type Options struct {
	Digest  string
	ImageID string
	Created int64
}

// SortKey returns a sort key (used to sort tags before process or display them)
func (tg *Tag) SortKey() string {
	return tg.GetCreatedKey() + tg.name
}

// Name gets tag name
func (tg *Tag) Name() string {
	return tg.name
}

// GetDigest gets tagged image's digest
func (tg *Tag) GetDigest() string {
	return tg.digest
}

// GetShortDigest gets shorter form of tagged image's digest
func (tg *Tag) GetShortDigest() string {
	const limit = 40

	if len(tg.digest) < limit {
		return tg.digest
	}

	return tg.digest[0:limit]
}

func cutImageID(s string) string {
	fields := strings.Split(s, ":")

	var id string
	if len(fields) < 2 {
		id = s
	} else {
		id = fields[1]
	}

	if len(id) > 12 {
		return id[0:12]
	}

	return id
}

// setImageID sets local Docker image ID
func (tg *Tag) setImageID(s string) {
	tg.imageID = cutImageID(s)
}

// GetImageID gets local Docker image ID
func (tg *Tag) GetImageID() string {
	return tg.imageID
}

// HasImageID tells us if Docker image has an ID defined
func (tg *Tag) HasImageID() bool {
	return len(tg.imageID) > 0
}

// setState sets tag state (a difference between local tag and its remote counterpart)
func (tg *Tag) setState(state string) {
	tg.state = state
}

// GetState gets tag state (a difference between local tag and its remote counterpart)
func (tg *Tag) GetState() string {
	return tg.state
}

// NeedsPull tells us if tag/image needs pull
func (tg *Tag) NeedsPull() bool {
	if tg.state == "ABSENT" || tg.state == "CHANGED" || tg.state == "ASSUMED" {
		return true
	}

	return false
}

// NeedsPush tells us if tag/image needs push to a registry
func (tg *Tag) NeedsPush(doUpdate bool) bool {
	if tg.state == "ABSENT" || tg.state == "ASSUMED" || (tg.state == "CHANGED" && doUpdate) {
		return true
	}

	return false
}

// GetCreated gets image creation timestamp
func (tg *Tag) GetCreated() int64 {
	return tg.created
}

// GetCreatedKey gets image creation timestamp in a string form (for a string sort e.g.)
func (tg *Tag) GetCreatedKey() string {
	return strconv.FormatInt(tg.created, 10)
}

// GetCreatedString gets image creation timestamp in a human-readable string form
func (tg *Tag) GetCreatedString() string {
	t := time.Unix(tg.created, 0)
	s := t.Format(time.RFC3339)
	p := strings.Split(s, "+")

	return p[0]
}

// New creates a new instance of Tag
func New(name string, options Options) (*Tag, error) {
	if name == "" {
		return nil, errors.New("Empty tag name not allowed")
	}

	if options.Digest == "" {
		return nil, errors.New("Empty image digest not allowed")
	}

	return &Tag{
			name:    name,
			digest:  options.Digest,
			imageID: cutImageID(options.ImageID),
			created: options.Created,
		},
		nil
}

func calculateState(name string, remoteTags, localTags map[string]*Tag) string {
	r, definedInRegistry := remoteTags[name]
	l, definedLocally := localTags[name]

	if definedInRegistry && !definedLocally {
		return "ABSENT"
	}

	if !definedInRegistry && definedLocally {
		return "LOCAL-ONLY"
	}

	if definedInRegistry && definedLocally {
		if r.GetDigest() == l.GetDigest() {
			return "PRESENT"
		}

		return "CHANGED"
	}

	return "ASSUMED"
}

// Join joins local tags with ones from registry, performs state processing and returns:
// * sorted slice of sort keys
// * joined map of [sortKey]name
// * joined map of [name]*Tag
func Join(
	remoteTags, localTags map[string]*Tag,
	assumedTagNames []string,
) ([]string, map[string]string, map[string]*Tag) {
	sortedKeys := make([]string, 0)
	tagNames := make(map[string]string)
	joinedTags := make(map[string]*Tag)

	for name := range remoteTags {
		sortKey := remoteTags[name].SortKey()

		sortedKeys = append(sortedKeys, sortKey)
		tagNames[sortKey] = name

		joinedTags[name] = remoteTags[name]

		ltg, defined := localTags[name]
		if defined && ltg.HasImageID() {
			joinedTags[name].setImageID(ltg.GetImageID())
		} else {
			joinedTags[name].setImageID("n/a")
		}
	}

	for name := range localTags {
		_, defined := remoteTags[name]
		if !defined {
			sortKey := localTags[name].SortKey()

			sortedKeys = append(sortedKeys, sortKey)
			tagNames[sortKey] = name

			joinedTags[name] = localTags[name]
		}
	}

	if assumedTagNames != nil {
		for _, name := range assumedTagNames {
			_, definedRemotely := remoteTags[name]
			_, definedLocally := localTags[name]

			if !definedRemotely && !definedLocally {
				joinedTags[name], _ = New(name, Options{Digest: "n/a", ImageID: "n/a"})

				sortKey := joinedTags[name].SortKey()

				sortedKeys = append(sortedKeys, sortKey)
				tagNames[sortKey] = name
			}
		}
	}

	for name, jtg := range joinedTags {
		jtg.setState(
			calculateState(
				name,
				remoteTags,
				localTags,
			),
		)
	}

	sort.Strings(sortedKeys)

	return sortedKeys, tagNames, joinedTags
}

// Collect organizes tags structures the way they could be used by API
func Collect(keys []string, tagNames map[string]string, tagMap map[string]*Tag) []*Tag {
	tags := make([]*Tag, len(keys))

	for i, key := range keys {
		name := tagNames[key]

		tags[i] = tagMap[name]
	}

	return tags
}
