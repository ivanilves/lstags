package collection

import (
	"fmt"

	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"
)

func refHasTags(ref string, tags map[string][]*tag.Tag) bool {
	for r := range tags {
		if r == ref {
			return true
		}
	}

	return false
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}

	return false
}

// New creates a new collection of API resources
func New(refs []string, tags map[string][]*tag.Tag) (*Collection, error) {
	repos := make(map[string]*repository.Repository)

	for _, ref := range refs {
		repo, err := repository.ParseRef(ref)
		if err != nil {
			return nil, err
		}

		repos[ref] = repo

		if !refHasTags(ref, tags) {
			return nil, fmt.Errorf("repository reference has no tags: %s", ref)
		}
	}

	for ref := range tags {
		if !contains(refs, ref) {
			return nil, fmt.Errorf("repository has tags, but not referenced: %s", ref)
		}
	}

	return &Collection{refs: refs, repos: repos, tags: tags}, nil
}

// Collection of API resources received from a registry or Docker daemon query
type Collection struct {
	refs  []string
	repos map[string]*repository.Repository
	tags  map[string][]*tag.Tag
}

// Refs returns all repository references from collection
func (cn *Collection) Refs() []string {
	return cn.refs
}

// Repos returns all repository structures from collection
func (cn *Collection) Repos() []*repository.Repository {
	repos := make([]*repository.Repository, cn.RepoCount())

	for i, ref := range cn.Refs() {
		repos[i] = cn.repos[ref]
	}

	return repos
}

// Repo returns repo structure, if it is present in collection (nil if not)
func (cn *Collection) Repo(ref string) *repository.Repository {
	for _, r := range cn.Refs() {
		if r == ref {
			return cn.repos[ref]
		}
	}

	return nil
}

// Tags returns slice of tag structures, if it is present in collection (nil if not)
func (cn *Collection) Tags(ref string) []*tag.Tag {
	repo := cn.Repo(ref)
	if repo == nil {
		return nil
	}

	return cn.tags[ref]
}

// TagMap returns [name]*Tag map of tag structures, if it is present in collection (nil if not)
func (cn *Collection) TagMap(ref string) map[string]*tag.Tag {
	tags := cn.Tags(ref)
	if tags == nil {
		return nil
	}

	tagMap := make(map[string]*tag.Tag)
	for _, tg := range tags {
		tagMap[tg.Name()] = tg
	}

	return tagMap
}

// RepoCount counts total repo number inside the collection
func (cn *Collection) RepoCount() int {
	return len(cn.refs)
}

// TagCount counts total tag number inside the collection
func (cn *Collection) TagCount() int {
	i := 0

	for _, v := range cn.tags {
		i += len(v)
	}

	return i
}
