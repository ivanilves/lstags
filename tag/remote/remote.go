package remote

import (
	"strings"
	"time"

	"github.com/ivanilves/lstags/repository"
	"github.com/ivanilves/lstags/tag"

	"github.com/ivanilves/lstags/api/v1/registry/client"
)

// ConcurrentRequests defines maximum number of concurrent requests we could maintain against the registry
var ConcurrentRequests = 32

// WaitBetween defines how much we will wait between batches of requests
var WaitBetween time.Duration

// RetryRequests is a number of retries we do in case of request failure
var RetryRequests = 0

// RetryDelay is a delay between retries of failed requests to the registry
var RetryDelay = 5 * time.Second

// TraceRequests defines if we should print out HTTP request URLs and response headers/bodies
var TraceRequests = false

func calculateBatchSteps(count, limit int) (int, int) {
	total := count / limit
	remain := count % limit

	if remain == 0 {
		return total, 0
	}

	return total + 1, remain
}

func calculateBatchStepSize(stepNumber, stepsTotal, remain, limit int) int {
	if remain != 0 && stepNumber == stepsTotal {
		return remain
	}

	return limit
}

// FetchTags looks up Docker repoPath tags present on remote Docker registry
func FetchTags(repo *repository.Repository, username, password string) (map[string]*tag.Tag, error) {
	cli, err := client.New(
		repo.Registry(),
		client.Config{
			ConcurrentRequests: ConcurrentRequests,
			WaitBetween:        WaitBetween,
			RetryRequests:      RetryRequests,
			RetryDelay:         RetryDelay,
			TraceRequests:      TraceRequests,
			IsInsecure:         !repo.IsSecure(),
		},
	)
	if err != nil {
		return nil, err
	}

	if err := cli.Login(username, password); err != nil {
		return nil, err
	}

	allTagNames, err := cli.TagNames(repo.Path())
	if err != nil {
		return nil, err
	}

	tagNames := make([]string, 0)
	for _, tagName := range allTagNames {
		if repo.MatchTag(tagName) {
			tagNames = append(tagNames, tagName)
		}
	}

	tags := make(map[string]*tag.Tag)

	batchSteps, batchRemain := calculateBatchSteps(len(tagNames), ConcurrentRequests)

	var stepSize int
	var tagIndex = 0
	for b := 1; b <= batchSteps; b++ {
		stepSize = calculateBatchStepSize(b, batchSteps, batchRemain, ConcurrentRequests)

		type response struct {
			Tag *tag.Tag
			Err error
		}

		rc := make(chan response, stepSize)

		for s := 1; s <= stepSize; s++ {
			go func(
				repo *repository.Repository,
				tagName string,
				rc chan response,
			) {
				tg, err := cli.Tag(repo.Path(), tagName)

				rc <- response{Tag: tg, Err: err}
			}(repo, tagNames[tagIndex], rc)

			tagIndex++

			time.Sleep(WaitBetween)
		}

		var i = 0
		for r := range rc {
			if r.Err != nil {
				if !strings.Contains(r.Err.Error(), "404 Not Found") {
					return nil, r.Err
				}
			} else {
				tags[r.Tag.Name()] = r.Tag
			}

			i++

			if i >= cap(rc) {
				close(rc)
			}
		}
	}

	return tags, nil
}
