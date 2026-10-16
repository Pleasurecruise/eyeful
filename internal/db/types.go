package db

import (
	"context"
	"encoding/json"
	"net/http"
)

type Queryer interface {
	Query(ctx context.Context, query string, params ...string) ([]Row, error)
	Exec(ctx context.Context, query string, params ...string) error
}

type Row struct {
	values map[string]*string
}

type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

type d1Error struct {
	msg string
}

type SingleQuery struct {
	SQL    string   `json:"sql"`
	Params []string `json:"params,omitempty"`
}

type QueryResponse struct {
	Success  bool           `json:"success"`
	Errors   []ResponseInfo `json:"errors"`
	Messages []ResponseInfo `json:"messages"`
	Result   []QueryResult  `json:"result"`
}

type ResponseInfo struct {
	Code             int             `json:"code"`
	Message          string          `json:"message"`
	DocumentationURL string          `json:"documentation_url,omitempty"`
	Source           *ResponseSource `json:"source,omitempty"`
}

type ResponseSource struct {
	Pointer string `json:"pointer,omitempty"`
}

type QueryResult struct {
	Meta    *QueryMeta                   `json:"meta,omitempty"`
	Results []map[string]json.RawMessage `json:"results,omitempty"`
	Success bool                         `json:"success,omitempty"`
}

type QueryMeta struct {
	ChangedDB       *bool         `json:"changed_db,omitempty"`
	Changes         *int64        `json:"changes,omitempty"`
	Duration        *float64      `json:"duration,omitempty"`
	LastRowID       *int64        `json:"last_row_id,omitempty"`
	RowsRead        *int64        `json:"rows_read,omitempty"`
	RowsWritten     *int64        `json:"rows_written,omitempty"`
	ServedByColo    string        `json:"served_by_colo,omitempty"`
	ServedByPrimary *bool         `json:"served_by_primary,omitempty"`
	ServedByRegion  string        `json:"served_by_region,omitempty"`
	SizeAfter       *int64        `json:"size_after,omitempty"`
	Timings         *QueryTimings `json:"timings,omitempty"`
}

type QueryTimings struct {
	SQLDurationMS *float64 `json:"sql_duration_ms,omitempty"`
}
