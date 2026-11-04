package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	tc "github.com/Roma7-7-7/telegram"
)

const (
	defaultAPIBaseURL = "https://api.telegram.org"
	maxErrorBodyBytes = 4 << 10
)

// PhotoClient sends photos via Bot API. The shared telegram client supports text only.
// It maps errors and runs middlewares the same way, so blocked users are purged for photos as well.
type PhotoClient struct {
	httpClient  *http.Client
	botToken    string
	baseURL     string
	middlewares []tc.MiddlewareFunc
}

type photoContext struct {
	context.Context

	chatID string
}

func (c *photoContext) ChatID() (string, bool) {
	return c.chatID, c.chatID != ""
}

func NewPhotoClient(httpClient *http.Client, botToken string, middlewares ...tc.MiddlewareFunc) *PhotoClient {
	return &PhotoClient{
		httpClient:  httpClient,
		botToken:    botToken,
		baseURL:     defaultAPIBaseURL,
		middlewares: middlewares,
	}
}

func (c *PhotoClient) SendPhoto(ctx context.Context, chatID string, photo []byte) error {
	var h tc.Handler = func(ctx tc.Context) error {
		return c.sendPhoto(ctx, chatID, photo)
	}
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h(&photoContext{Context: ctx, chatID: chatID})
}

func (c *PhotoClient) sendPhoto(ctx context.Context, chatID string, photo []byte) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("chat_id", chatID); err != nil {
		return fmt.Errorf("write chat_id: %w", err)
	}
	part, err := w.CreateFormFile("photo", "schedule.png")
	if err != nil {
		return fmt.Errorf("create photo part: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("write photo: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/bot%s/sendPhoto", c.baseURL, c.botToken), &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// url.Error text contains the request URL, which embeds the bot token
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	//nolint:mnd // 1xx and 2xx status codes are ok
	if resp.StatusCode < 300 {
		return nil
	}

	switch resp.StatusCode {
	case http.StatusForbidden:
		return tc.ErrForbidden
	case http.StatusTooManyRequests:
		if retryAfter := parseAPIError(resp.Body).Parameters.RetryAfter; retryAfter > 0 {
			return fmt.Errorf("%w: retry after %ds", tc.ErrTooManyRequests, retryAfter)
		}
		return tc.ErrTooManyRequests
	default:
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, parseAPIError(resp.Body).Description)
	}
}

type apiError struct {
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func parseAPIError(body io.Reader) apiError {
	var e apiError
	_ = json.NewDecoder(io.LimitReader(body, maxErrorBodyBytes)).Decode(&e)
	return e
}
