package monday

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
)

// FileAsset is the result of uploading a file to monday.
type FileAsset struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	PublicURL string `json:"public_url,omitempty"`
}

// AddFileToUpdate uploads a file to an update (comment) using monday's
// GraphQL multipart file endpoint. The caller provides the file contents so the
// server process never has to resolve a path for the client.
func (c *Client) AddFileToUpdate(ctx context.Context, updateID, filename string, content []byte) (*FileAsset, error) {
	query := `mutation ($file: File!, $updateId: ID!) { add_file_to_update(update_id: $updateId, file: $file) { id name url public_url } }`
	variables := map[string]any{"updateId": updateID, "file": nil}
	return c.uploadFile(ctx, query, variables, "add_file_to_update", filename, content)
}

// AddFileToColumn uploads a file to a file-type column of an item.
func (c *Client) AddFileToColumn(ctx context.Context, itemID, columnID, filename string, content []byte) (*FileAsset, error) {
	query := `mutation ($file: File!, $itemId: ID!, $columnId: String!) { add_file_to_column(item_id: $itemId, column_id: $columnId, file: $file) { id name url public_url } }`
	variables := map[string]any{"itemId": itemID, "columnId": columnID, "file": nil}
	return c.uploadFile(ctx, query, variables, "add_file_to_column", filename, content)
}

// uploadFile performs a GraphQL multipart request (graphql-multipart-request
// spec): an "operations" part with the query and variables (the file variable
// left null), a "map" part pointing the file part at that variable, and the
// file part itself.
func (c *Client) uploadFile(ctx context.Context, query string, variables map[string]any, field, filename string, content []byte) (*FileAsset, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("file content is empty")
	}
	apiToken := c.apiToken
	if c.requestAuth {
		var ok bool
		apiToken, ok = requestToken(ctx)
		if !ok {
			return nil, fmt.Errorf("monday request requires a bearer credential")
		}
	}

	operations, err := json.Marshal(GraphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("marshal operations: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("query", string(operations)); err != nil {
		return nil, fmt.Errorf("write query field: %w", err)
	}
	// Map the "variables.file" (0) part to the file upload.
	if err := writer.WriteField("map", `{"0":["variables.file"]}`); err != nil {
		return nil, fmt.Errorf("write map field: %w", err)
	}
	part, err := writer.CreateFormFile("0", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, fmt.Errorf("write file part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.fileURL(), bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Authorization", apiToken)
	req.Header.Set("API-Version", c.apiVersion)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("monday file upload failed: %s", redactToken(err.Error(), apiToken))
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, c.maxResponseBytes+1)
	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, fmt.Errorf("read upload response: %w", readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: response.StatusCode,
			Message:    http.StatusText(response.StatusCode),
			RetryAfter: retryAfter(response.Header.Get("Retry-After")),
			Temporary:  response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
		}
	}

	var envelope struct {
		Data map[string]struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			URL       string `json:"url"`
			PublicURL string `json:"public_url"`
		} `json:"data"`
		Errors []GraphQLError `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode upload response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return nil, newRequestError(envelope.Errors[0])
	}
	asset, ok := envelope.Data[field]
	if !ok || asset.ID == "" {
		return nil, fmt.Errorf("monday file upload returned no asset for %s", field)
	}
	return &FileAsset{ID: asset.ID, Name: asset.Name, URL: asset.URL, PublicURL: asset.PublicURL}, nil
}

// fileURL derives monday's file endpoint from the GraphQL API URL
// (https://api.monday.com/v2 -> https://api.monday.com/v2/file).
func (c *Client) fileURL() string {
	return c.apiURL + "/file"
}
