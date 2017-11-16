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
	name        string
	digest      string
	imageID     string
	state       string
	created     int64
	containerID string
}

// SortKey returns a sort key
func (tg *Tag) SortKey() string {
	return tg.GetCreatedKey() + tg.name
}

// GetName gets tag name
func (tg *Tag) GetName() string {
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

func calculateImageID(s string) string {
	fields := strings.Split(s, ":")

	if len(fields) < 2 {
		return s
	}

	if len(fields[1]) > 12 {
		return fields[1][0:12]
	}

	return fields[1]
}

// SetImageID sets local Docker image ID
func (tg *Tag) SetImageID(s string) {
	tg.imageID = calculateImageID(s)
}

// GetImageID gets local Docker image ID
func (tg *Tag) GetImageID() string {
	return tg.imageID
}

// HasImageID tells us if Docker image has an ID defined
func (tg *Tag) HasImageID() bool {
	return len(tg.imageID) > 0
}

// SetState sets repo tag state
func (tg *Tag) SetState(state string) {
	tg.state = state
}

// GetState gets repo tag state
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

// SetCreated sets image creation timestamp
func (tg *Tag) SetCreated(created int64) {
	tg.created = created
}

// GetCreated gets image creation timestamp
func (tg *Tag) GetCreated() int64 {
	return tg.created
}

// SetContainerID sets "container ID": an OAF "image digest" generated locally
func (tg *Tag) SetContainerID(containerID string) {
	tg.containerID = containerID
}

// GetContainerID gets "container ID": an OAF "image digest" generated locally
func (tg *Tag) GetContainerID() string {
	return tg.containerID
}

// HasContainerID tells us if tag has "container ID"
func (tg *Tag) HasContainerID() bool {
	return len(tg.containerID) > 0
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
func New(name, digest string) (*Tag, error) {
	if name == "" {
		return nil, errors.New("Empty tag name not allowed")
	}

	if digest == "" {
		return nil, errors.New("Empty image digest not allowed")
	}

	return &Tag{
			name:   name,
			digest: digest,
		},
		nil
}

func calculateState(name string, remoteTags, localTags map[string]*Tag) string {
	r, definedInRegistry := remoteTags[name]
	l, definedLocally := localTags[name]

	if !definedInRegistry && !definedLocally {
		return "ASSUMED"
	}

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

		if r.HasContainerID() && l.HasContainerID() {
			if r.GetContainerID() == l.GetContainerID() {
				return "PRESENT"
			}
		}

		return "CHANGED"
	}

	return "UNKNOWN"
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
	names := make(map[string]string)
	joinedTags := make(map[string]*Tag)

	for name := range remoteTags {
		sortKey := remoteTags[name].SortKey()

		sortedKeys = append(sortedKeys, sortKey)
		names[sortKey] = name

		joinedTags[name] = remoteTags[name]

		ltg, defined := localTags[name]
		if defined && ltg.HasImageID() {
			joinedTags[name].SetImageID(ltg.GetImageID())
		} else {
			joinedTags[name].SetImageID("n/a")
		}
	}

	for name := range localTags {
		_, defined := remoteTags[name]
		if !defined {
			sortKey := localTags[name].SortKey()

			sortedKeys = append(sortedKeys, sortKey)
			names[sortKey] = name

			joinedTags[name] = localTags[name]
		}
	}

	if assumedTagNames != nil {
		for _, name := range assumedTagNames {
			_, definedRemotely := remoteTags[name]
			_, definedLocally := localTags[name]

			if !definedRemotely && !definedLocally {
				joinedTags[name], _ = New(name, "n/a")

				joinedTags[name].SetImageID("n/a")

				sortKey := joinedTags[name].SortKey()

				sortedKeys = append(sortedKeys, sortKey)
				names[sortKey] = name
			}
		}
	}

	for name, jtg := range joinedTags {
		jtg.SetState(
			calculateState(
				name,
				remoteTags,
				localTags,
			),
		)
	}

	sort.Strings(sortedKeys)

	return sortedKeys, names, joinedTags
}

// Collection encapsulates collection of tags received from a registry/repository query
type Collection struct {
	Registry   string
	RepoName   string
	RepoPath   string
	Tags       []*Tag
	PullTags   []*Tag
	PushTags   []*Tag
	PushPrefix string
}
