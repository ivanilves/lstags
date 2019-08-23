// Package client provides Docker registry client API
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ivanilves/lstags/api/v1/registry/client/auth"
	"github.com/ivanilves/lstags/api/v1/registry/client/request"
	"github.com/ivanilves/lstags/tag"
)

// DefaultConcurrentRequests will be used if no explicit ConcurrentRequests configured
var DefaultConcurrentRequests = 32

// DefaultRetryDelay will be used if no explicit RetryDelay configured
var DefaultRetryDelay = 30 * time.Second

// MaxConcurrentRequests is a hard limit for simultaneous registry requests
const MaxConcurrentRequests = 256

// RegistryClient is an abstraction to wrap logic of working with Docker registry
// incl. connection, authentification, authorization, obtaining information etc...
type RegistryClient struct {
	registry string
	username string
	password string

	// Config has general configuration of the registry client instance
	Config Config
	// Token is an authentication token obtained after registry login
	Token auth.Token
	// RepoTokens are per-repo tokens (make sense for "Bearer" authentication only)
	RepoTokens map[string]auth.Token
}

// Config has configuration parameters for RegistryClient creation
type Config struct {
	// ConcurrentRequests defines how much requests to registry we could run concurrently
	ConcurrentRequests int
	// WaitBetween defines how much we will wait between batches of requests
	WaitBetween time.Duration
	// RetryRequests defines how much retries we will do to the failed HTTP request
	RetryRequests int
	// RetryDelay defines how much we will wait between failed HTTP request and retry
	RetryDelay time.Duration
	// TraceRequests sets if we will print out registry HTTP request traces
	TraceRequests bool
	// IsInsecure sets if we want to communicate registry over plain HTTP instead of HTTPS
	IsInsecure bool
}

// New creates and validates new RegistryClient instance
func New(registry string, config Config) (*RegistryClient, error) {
	if config.ConcurrentRequests == 0 {
		config.ConcurrentRequests = DefaultConcurrentRequests
	}

	if config.RetryDelay == 0 {
		config.RetryDelay = DefaultRetryDelay
	}

	if config.ConcurrentRequests > MaxConcurrentRequests {
		err := fmt.Errorf(
			"Could not run more than %d concurrent requests (%d configured)",
			MaxConcurrentRequests,
			config.ConcurrentRequests,
		)

		return nil, err
	}

	return &RegistryClient{
		registry:   registry,
		Config:     config,
		RepoTokens: make(map[string]auth.Token),
	}, nil
}

func (cli *RegistryClient) webScheme() string {
	if cli.Config.IsInsecure {
		return "http://"
	}

	return "https://"
}

// URL formats a valid URL for the V2 registry
func (cli *RegistryClient) URL() string {
	return cli.webScheme() + cli.registry + "/v2/"
}

// Ping checks basic connectivity to the registry
func (cli *RegistryClient) Ping() error {
	resp, err := http.Get(cli.URL())
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 401 {
		return fmt.Errorf("Unexpected status: %s", resp.Status)
	}

	return nil
}

// Login logs in to the registry (returns error, if failed)
func (cli *RegistryClient) Login(username, password string) error {
	tk, err := auth.NewToken(cli.URL(), username, password, "registry:catalog:*")
	if err != nil {
		log.Debugf("Try to login with less permissions (repository:catalog:*)")
		tk, err = auth.NewToken(cli.URL(), username, password, "repository:catalog:*")

		if err != nil {
			if username == "" && password == "" {
				return nil
			}

			return err
		}
	}

	cli.Token = tk

	cli.username = username
	cli.password = password

	return nil
}

// IsLoggedIn indicates if we are logged in to registry or not
func (cli *RegistryClient) IsLoggedIn() bool {
	return cli.Token != nil
}

func decodeTagData(body io.ReadCloser) ([]string, map[string]tag.Manifest, error) {
	tagData := struct {
		TagNames     []string                `json:"tags"`
		TagManifests map[string]tag.Manifest `json:"manifest,omitempty"`
	}{}

	err := json.NewDecoder(body).Decode(&tagData)
	if err != nil {
		return nil, nil, err
	}

	tagManifests := make(map[string]tag.Manifest)

	for _, manifest := range tagData.TagManifests {
		for _, tagName := range manifest.Tags {
			tagManifests[tagName] = manifest
		}
	}

	return tagData.TagNames, tagManifests, nil
}

func (cli *RegistryClient) repoToken(repoPath string) (auth.Token, error) {
	if cli.Token != nil && cli.Token.Method() != "Bearer" {
		return cli.Token, nil
	}

	_, tokenDefined := cli.RepoTokens[repoPath]
	if tokenDefined {
		return cli.RepoTokens[repoPath], nil
	}

	repoToken, err := auth.NewToken(
		cli.URL(),
		cli.username,
		cli.password,
		"repository:"+repoPath+":pull",
	)
	if err != nil {
		return nil, err
	}

	cli.RepoTokens[repoPath] = repoToken

	return repoToken, nil
}

func mergeTagManifests(a, b map[string]tag.Manifest) map[string]tag.Manifest {
	if b == nil {
		return a
	}

	for k, v := range b {
		a[k] = v
	}

	return a
}

// TagData gets list of all tag names and all additional data for the repository path specified
func (cli *RegistryClient) TagData(repoPath string) ([]string, map[string]tag.Manifest, error) {
	repoToken, err := cli.repoToken(repoPath)
	if err != nil {
		return nil, nil, err
	}

	allTagNames := make([]string, 0)
	allTagManifests := make(map[string]tag.Manifest)

	link := "/tags/list"
	for {
		resp, nextlink, err := request.Perform(
			cli.URL()+repoPath+link,
			repoToken.Method()+" "+repoToken.String(),
			"v2",
			cli.Config.TraceRequests,
			cli.Config.RetryRequests,
			cli.Config.RetryDelay,
		)
		if err != nil {
			return nil, nil, err
		}

		tagNames, tagManifests, err := decodeTagData(resp.Body)
		if err != nil {
			return nil, nil, err
		}

		allTagNames = append(allTagNames, tagNames...)
		allTagManifests = mergeTagManifests(allTagManifests, tagManifests)

		if nextlink == "" {
			break
		}

		link = "/tags/list?" + nextlink
	}

	return allTagNames, allTagManifests, nil
}

func (cli *RegistryClient) tagDigest(repoPath, tagName string) (string, error) {
	repoToken, err := cli.repoToken(repoPath)
	if err != nil {
		return "", err
	}

	resp, _, err := request.Perform(
		cli.URL()+repoPath+"/manifests/"+tagName,
		repoToken.Method()+" "+repoToken.String(),
		"v2",
		cli.Config.TraceRequests,
		cli.Config.RetryRequests,
		cli.Config.RetryDelay,
	)
	if err != nil {
		return "", err
	}

	type configField struct {
		Digest string `json:"digest"`
	}
	var values struct {
		Config configField `json:"config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&values); err != nil {
		return "", err
	}

	if values.Config.Digest == "" {
		values.Config.Digest = "this.image.is.bad.it.has.no.digest.fuuu!"
	}

	return values.Config.Digest, nil
}

func (cli *RegistryClient) v1TagHistory(s string) (*tag.Options, error) {
	var v1history struct {
		Created     string `json:"created"`
		ContainerID string `json:"container"`
	}

	err := json.Unmarshal([]byte(s), &v1history)
	if err != nil {
		return nil, err
	}

	t, err := time.Parse(time.RFC3339, v1history.Created)
	if err != nil {
		return nil, err
	}

	return &tag.Options{Created: t.Unix(), ImageID: v1history.ContainerID}, nil
}

func (cli *RegistryClient) v1TagOptions(repoPath, tagName string) (*tag.Options, error) {
	repoToken, err := cli.repoToken(repoPath)
	if err != nil {
		return nil, err
	}

	resp, _, err := request.Perform(
		cli.URL()+repoPath+"/manifests/"+tagName,
		repoToken.Method()+" "+repoToken.String(),
		"v1",
		cli.Config.TraceRequests,
		cli.Config.RetryRequests,
		cli.Config.RetryDelay,
	)
	if err != nil {
		return nil, err
	}

	var v1manifest struct {
		History []map[string]string `json:"history"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&v1manifest); err != nil {
		return nil, err
	}

	if len(v1manifest.History) == 0 {
		return nil, fmt.Errorf("no v1 history to extract data from")
	}

	return cli.v1TagHistory(v1manifest.History[0]["v1Compatibility"])
}

// Tag gets information about specified repository tag
func (cli *RegistryClient) Tag(repoPath, tagName string, tagManifest tag.Manifest) (*tag.Tag, error) {
	dc := make(chan string, 0)
	ec := make(chan error, 0)

	go func(dc chan string, ec chan error) {
		digest, err := cli.tagDigest(repoPath, tagName)
		if err != nil {
			ec <- err
			return
		}

		dc <- digest
	}(dc, ec)

	options, err := cli.v1TagOptions(repoPath, tagName)
	if err != nil {
		log.Debugf("%s\n", err.Error())

		options = &tag.Options{}
	}

	select {
	case digest := <-dc:
		options.Digest = digest
	case err := <-ec:
		return nil, err
	}

	if options.Created == 0 {
		options.Created = extractCreated(tagManifest.TimeCreatedMs, tagManifest.TimeUploadedMs)
	}

	return tag.New(tagName, *options)
}

func extractCreated(c, u string) int64 {
	created, err := strconv.ParseInt(c, 10, 64)
	if err == nil && created != 0 {
		return created / 1000
	}

	updated, err := strconv.ParseInt(u, 10, 64)
	if err == nil && updated != 0 {
		return updated / 1000
	}

	return 0
}
