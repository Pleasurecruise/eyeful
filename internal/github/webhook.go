package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

const (
	actionOpened         = "opened"
	actionSynchronize    = "synchronize"
	actionReopened       = "reopened"
	actionReadyForReview = "ready_for_review"
)

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	refPattern  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
)

type payload struct {
	Action       string        `json:"action"`
	Installation *installation `json:"installation"`
	PullRequest  *pullRequest  `json:"pull_request"`
	Repository   *repository   `json:"repository"`
}

type installation struct {
	ID int64 `json:"id"`
}

type pullRequest struct {
	Number int64  `json:"number"`
	Draft  bool   `json:"draft"`
	Head   gitRef `json:"head"`
	Base   gitRef `json:"base"`
}

type gitRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type repository struct {
	Name     string `json:"name"`
	CloneURL string `json:"clone_url"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func Parse(event string, body []byte) (Delivery, error) {
	switch event {
	case "ping":
		return Delivery{Ignore: "ping"}, nil
	case "pull_request":
	default:
		return Delivery{Ignore: "event " + event}, nil
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Delivery{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	switch p.Action {
	case actionOpened, actionSynchronize, actionReopened, actionReadyForReview:
	default:
		return Delivery{Ignore: "action " + p.Action}, nil
	}
	if p.PullRequest != nil && p.PullRequest.Draft && p.Action != actionReadyForReview {
		return Delivery{Ignore: "draft"}, nil
	}
	pull, err := pullFrom(p)
	if err != nil {
		return Delivery{}, err
	}
	return Delivery{Pull: pull}, nil
}

func Admission(p Pull) review.Request {
	return review.Request{
		Source: review.Source{
			Repo: p.Owner + "/" + p.Name, Base: p.BaseRef, Head: p.HeadRef,
			Number: p.Number, BaseSHA: p.BaseSHA, HeadSHA: p.HeadSHA,
			CloneURL: p.CloneURL, InstallationID: p.InstallationID,
		},
		IdempotencyKey: fmt.Sprintf("%s/%s#%d@%s", p.Owner, p.Name, p.Number, p.HeadSHA),
	}
}

func pullFrom(p payload) (Pull, error) {
	if p.Repository == nil || p.PullRequest == nil {
		return Pull{}, fmt.Errorf("%w: missing repository or pull_request", ErrMalformed)
	}
	if p.Installation == nil || p.Installation.ID < 1 {
		return Pull{}, fmt.Errorf("%w: missing installation", ErrMalformed)
	}
	pr := p.PullRequest
	owner, name := p.Repository.Owner.Login, p.Repository.Name
	if !namePattern.MatchString(owner) || !namePattern.MatchString(name) {
		return Pull{}, fmt.Errorf("%w: invalid repository name", ErrMalformed)
	}
	if pr.Number < 1 {
		return Pull{}, fmt.Errorf("%w: invalid pull request number", ErrMalformed)
	}
	if !shaPattern.MatchString(pr.Head.SHA) || !shaPattern.MatchString(pr.Base.SHA) {
		return Pull{}, fmt.Errorf("%w: invalid commit sha", ErrMalformed)
	}
	if !safeRef(pr.Base.Ref) || !safeRef(pr.Head.Ref) {
		return Pull{}, fmt.Errorf("%w: invalid branch ref", ErrMalformed)
	}
	if !safeCloneURL(p.Repository.CloneURL) {
		return Pull{}, fmt.Errorf("%w: invalid clone url", ErrMalformed)
	}
	return Pull{
		Owner: owner, Name: name, Number: pr.Number, Draft: pr.Draft,
		BaseRef: pr.Base.Ref, HeadRef: pr.Head.Ref, BaseSHA: pr.Base.SHA, HeadSHA: pr.Head.SHA,
		CloneURL: p.Repository.CloneURL, InstallationID: p.Installation.ID,
	}, nil
}

func safeRef(value string) bool {
	if value == "" || len(value) > 255 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".lock") {
		return false
	}
	if strings.Contains(value, "..") || strings.Contains(value, "//") || strings.Contains(value, "@{") {
		return false
	}
	return refPattern.MatchString(value)
}

func safeCloneURL(value string) bool {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return false
	}
	return true
}
