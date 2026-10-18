package review

type Source struct {
	Repo           string `json:"repo"`
	Base           string `json:"base"`
	Head           string `json:"head"`
	Number         int64  `json:"number"`
	BaseSHA        string `json:"base_sha"`
	HeadSHA        string `json:"head_sha"`
	CloneURL       string `json:"clone_url"`
	InstallationID int64  `json:"installation_id"`
}
