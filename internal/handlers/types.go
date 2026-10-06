package handlers

import (
	"context"
	"time"

	"github.com/google/go-github/v92/github"

	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/review"
)

type RepositoryReader interface {
	List(ctx context.Context, userID string) ([]*github.Installation, []*github.Repository, error)
	Get(ctx context.Context, userID, owner, name string) (*github.Repository, error)
	Commit(ctx context.Context, userID, owner, name, ref string) (*github.RepositoryCommit, error)
	Tree(ctx context.Context, userID, owner, name, sha string) (*github.Tree, error)
	Diff(ctx context.Context, userID, owner, name, sha string) (string, error)
}

type InstallationAccountResponse struct {
	Login string `json:"login" validate:"required" example:"octo"`
} // @name InstallationAccount

type InstallationResponse struct {
	ID                  int64                        `json:"id" validate:"required"`
	Account             *InstallationAccountResponse `json:"account,omitempty"`
	RepositorySelection string                       `json:"repository_selection" validate:"required" enums:"all,selected"`
	HTMLURL             string                       `json:"html_url" validate:"required" example:"https://github.com/settings/installations/1"`
	TargetType          string                       `json:"target_type" validate:"required" example:"User"`
} // @name Installation

type RepositoryList struct {
	Items         []RepositoryResponse   `json:"items" validate:"required"`
	Installations []InstallationResponse `json:"installations" validate:"required"`
} // @name RepositoryList

type OwnerResponse struct {
	Login string `json:"login" validate:"required" example:"octo"`
} // @name RepositoryOwner

type RepositoryResponse struct {
	Owner         OwnerResponse `json:"owner" validate:"required"`
	Name          string        `json:"name" validate:"required" example:"app"`
	FullName      string        `json:"full_name" validate:"required" example:"octo/app"`
	Private       bool          `json:"private" validate:"required"`
	HTMLURL       string        `json:"html_url" validate:"required"`
	DefaultBranch string        `json:"default_branch" validate:"required" example:"main"`
} // @name Repository

type GitUserResponse struct {
	Name  string     `json:"name,omitempty"`
	Email string     `json:"email,omitempty"`
	Date  *time.Time `json:"date,omitempty"`
} // @name GitUser

type TreeRefResponse struct {
	SHA string `json:"sha" validate:"required"`
} // @name TreeRef

type GitCommitResponse struct {
	Author    *GitUserResponse `json:"author,omitempty"`
	Committer *GitUserResponse `json:"committer,omitempty"`
	Message   string           `json:"message" validate:"required"`
	Tree      TreeRefResponse  `json:"tree" validate:"required"`
} // @name GitCommit

type ParentResponse struct {
	SHA string `json:"sha" validate:"required"`
} // @name CommitParent

type CommitResponse struct {
	SHA     string            `json:"sha" validate:"required"`
	HTMLURL string            `json:"html_url" validate:"required"`
	Commit  GitCommitResponse `json:"commit" validate:"required"`
	Parents []ParentResponse  `json:"parents" validate:"required"`
} // @name Commit

type GitTreeEntryResponse struct {
	Path string `json:"path" validate:"required"`
	Mode string `json:"mode" validate:"required"`
	Type string `json:"type" validate:"required" example:"blob"`
	SHA  string `json:"sha" validate:"required"`
	Size *int   `json:"size,omitempty"`
} // @name GitTreeEntry

type GitTreeResponse struct {
	SHA       string                 `json:"sha" validate:"required"`
	Truncated bool                   `json:"truncated" validate:"required"`
	Tree      []GitTreeEntryResponse `json:"tree" validate:"required"`
} // @name GitTree

type CommitDiffResponse struct {
	Diff string `json:"diff" validate:"required"`
} // @name CommitDiff

type APIKeyStore interface {
	CreateAPIKey(ctx context.Context, userID, name string) (auth.APIKey, string, error)
	ListAPIKeys(ctx context.Context, userID string) ([]auth.APIKey, error)
	GetAPIKey(ctx context.Context, userID, id string) (auth.APIKey, error)
	UpdateAPIKey(ctx context.Context, userID, id, name string) (auth.APIKey, error)
	DeleteAPIKey(ctx context.Context, userID, id string) error
}

type CreateAPIKeyRequest struct {
	Name string `json:"name" validate:"required" example:"ci"`
} // @name CreateAPIKeyRequest

type UpdateAPIKeyRequest struct {
	Name string `json:"name" validate:"required" example:"ci-main"`
} // @name UpdateAPIKeyRequest

type APIKeyResponse struct {
	ID         string     `json:"id" validate:"required" example:"key_4n8q2w6e1r5t9y3u"`
	Name       string     `json:"name" validate:"required"`
	Prefix     string     `json:"prefix" validate:"required" example:"eyf_a1b2c3"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at" validate:"required"`
} // @name APIKey

type CreatedAPIKey struct {
	Key    APIKeyResponse `json:"key" validate:"required"`
	Secret string         `json:"secret" validate:"required" example:"eyf_…"`
} // @name CreatedAPIKey

type APIKeyList struct {
	Items []APIKeyResponse `json:"items" validate:"required"`
} // @name APIKeyList

type HealthResponse struct {
	Status  string `json:"status" example:"ok" validate:"required"`
	Version string `json:"version" example:"0.1.0" validate:"required"`
} // @name HealthResponse

type OAuthProvider interface {
	Provider() auth.Provider
	AuthCodeURL(state, verifier string) string
	Callback(ctx context.Context, code, verifier string, c auth.ClientInfo) (auth.User, auth.SessionToken, error)
}

type OAuthProviderResponse struct {
	ID    string `json:"id" enums:"github,gitee" validate:"required" example:"github"`
	Title string `json:"title" validate:"required" example:"GitHub"`
} // @name OAuthProvider

type OAuthProviderList struct {
	Items []OAuthProviderResponse `json:"items" validate:"required"`
} // @name OAuthProviderList

type ReviewStore interface {
	Create(ctx context.Context, req review.Request) (review.Review, bool, error)
	Get(ctx context.Context, userID, id string) (review.Review, error)
	List(ctx context.Context, userID string, limit int32) ([]review.Review, error)
	Delete(ctx context.Context, userID, id string) error
}

type SourceBody struct {
	Repo string `json:"repo" example:"octo/app" validate:"required"`
	Base string `json:"base" example:"main" validate:"required"`
	Head string `json:"head" example:"feature/login" validate:"required"`
} // @name Source

type CreateReviewRequest struct {
	Source SourceBody `json:"source" validate:"required"`
	Note   string     `json:"note,omitempty" example:"focus on auth"`
} // @name CreateReviewRequest

type ReviewResponse struct {
	ID         string     `json:"id" example:"r_9f2c1a7b3e5d4c60" validate:"required"`
	Source     SourceBody `json:"source" validate:"required"`
	Note       string     `json:"note,omitempty"`
	Status     string     `json:"status" enums:"queued,running,done" validate:"required"`
	Result     string     `json:"result,omitempty" enums:"complete,none"`
	Reason     string     `json:"reason,omitempty" example:"the review pipeline is not implemented yet"`
	Attempts   int32      `json:"attempts" validate:"required"`
	CreatedAt  time.Time  `json:"created_at" validate:"required"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
} // @name Review

type ReviewList struct {
	Items []ReviewResponse `json:"items" validate:"required"`
} // @name ReviewList

type SessionStore interface {
	ListSessions(ctx context.Context, userID string) ([]auth.Session, error)
	DeleteSession(ctx context.Context, userID, id string) error
	DeleteSessionByToken(ctx context.Context, token string) error
}

type SessionResponse struct {
	ID        string    `json:"id" validate:"required" example:"ses_7d2k9m4p1q8r3t6v"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
	ExpiresAt time.Time `json:"expires_at" validate:"required"`
} // @name Session

type SessionList struct {
	Items []SessionResponse `json:"items" validate:"required"`
} // @name SessionList

type CurrentSession struct {
	User UserResponse `json:"user" validate:"required"`
	Via  string       `json:"via" enums:"session,api_key" validate:"required"`
} // @name CurrentSession

type UserStore interface {
	UpdateUser(ctx context.Context, userID, name string) (auth.User, error)
	DeleteUser(ctx context.Context, userID string) error
}

type UpdateUserRequest struct {
	Name string `json:"name" validate:"required" example:"Ada Lovelace"`
} // @name UpdateUserRequest

type UserResponse struct {
	ID        string    `json:"id" validate:"required" example:"usr_k3x9p2m4q8r1t6v0"`
	Email     string    `json:"email" validate:"required"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at" validate:"required"`
} // @name User
