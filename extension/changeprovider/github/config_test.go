package github

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	baseURL := "https://api.github.com"

	client := NewClient(httpClient, baseURL)

	assert.Equal(t, httpClient, client.HTTPClient())
	assert.Equal(t, baseURL, client.BaseURL())
	assert.Equal(t, "https://api.github.com/graphql", client.GraphQLURL())
}

func TestClient_BaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "GitHub.com",
			baseURL: "https://api.github.com",
			want:    "https://api.github.com",
		},
		{
			name:    "GitHub Enterprise Server",
			baseURL: "https://ghe.company.com/api",
			want:    "https://ghe.company.com/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(&http.Client{}, tt.baseURL)
			assert.Equal(t, tt.want, client.BaseURL())
		})
	}
}

func TestClient_GraphQLURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "GitHub.com",
			baseURL: "https://api.github.com",
			want:    "https://api.github.com/graphql",
		},
		{
			name:    "GitHub Enterprise Server",
			baseURL: "https://ghe.company.com/api",
			want:    "https://ghe.company.com/api/graphql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(&http.Client{}, tt.baseURL)
			assert.Equal(t, tt.want, client.GraphQLURL())
		})
	}
}

func TestClient_RESTURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		path    string
		want    string
	}{
		{
			name:    "GitHub.com - pull request",
			baseURL: "https://api.github.com",
			path:    "/repos/uber/submitqueue/pulls/123",
			want:    "https://api.github.com/repos/uber/submitqueue/pulls/123",
		},
		{
			name:    "GitHub Enterprise Server - pull request",
			baseURL: "https://ghe.company.com/api",
			path:    "/repos/uber/submitqueue/pulls/456",
			want:    "https://ghe.company.com/api/repos/uber/submitqueue/pulls/456",
		},
		{
			name:    "repos endpoint",
			baseURL: "https://api.github.com",
			path:    "/repos/uber/submitqueue",
			want:    "https://api.github.com/repos/uber/submitqueue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(&http.Client{}, tt.baseURL)
			assert.Equal(t, tt.want, client.RESTURL(tt.path))
		})
	}
}
