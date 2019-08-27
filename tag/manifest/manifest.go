package manifest

import (
	"strconv"
)

// Manifest is an additional tag information presented by some registries (e.g. GCR)
type Manifest struct {
	ID             string
	ImageSizeBytes int64
	MediaType      string
	Tags           []string
	TimeCreated    int64
	TimeUploaded   int64
}

// Created gets image/tag creation date
func (m Manifest) Created() int64 {
	if m.TimeCreated != 0 {
		return m.TimeCreated
	}

	return m.TimeUploaded
}

// Raw embodies raw, unprocessed manifest structure
type Raw struct {
	ImageSizeBytes string
	MediaType      string
	Tags           []string `json:"tag"`
	TimeCreatedMs  string
	TimeUploadedMs string
}

// Parse parses raw manifest and returns a normal one
func Parse(id string, r Raw) (*Manifest, error) {
	imageSizeBytes, err := strconv.ParseInt(r.ImageSizeBytes, 10, 64)
	if err != nil {
		return nil, err
	}

	timeCreated, err := strconv.ParseInt(r.TimeCreatedMs, 10, 64)
	if err != nil {
		return nil, err
	}
	timeCreated = timeCreated / 1000

	timeUploaded, err := strconv.ParseInt(r.TimeUploadedMs, 10, 64)
	if err != nil {
		return nil, err
	}
	timeUploaded = timeUploaded / 1000

	return &Manifest{
		ID:             id,
		ImageSizeBytes: imageSizeBytes,
		MediaType:      r.MediaType,
		Tags:           r.Tags,
		TimeCreated:    timeCreated,
		TimeUploaded:   timeUploaded,
	}, nil
}

// ParseMap does Parse() over a map of passed raw manifests
func ParseMap(rs map[string]Raw) (map[string]Manifest, error) {
	manifests := make(map[string]Manifest)

	for k, v := range rs {
		m, err := Parse(k, v)
		if err != nil {
			return nil, err
		}

		manifests[k] = *m
	}

	return manifests, nil
}

// MapByTag maps passed manifests by their tag names
// By default manifests are mapped by their digests.
func MapByTag(manifests map[string]Manifest) map[string]Manifest {
	mappedByTag := make(map[string]Manifest)

	for _, v := range manifests {
		for _, tagName := range v.Tags {
			mappedByTag[tagName] = v
		}
	}

	return mappedByTag
}

// MergeMaps merges two passed manifest maps into a single one
func MergeMaps(mapA, mapB map[string]Manifest) map[string]Manifest {
	if mapB == nil {
		return mapA
	}

	for k, v := range mapB {
		mapA[k] = v
	}

	return mapA
}
