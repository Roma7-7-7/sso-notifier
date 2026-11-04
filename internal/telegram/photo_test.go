package telegram

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tc "github.com/Roma7-7-7/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhotoClient_SendPhoto(t *testing.T) {
	t.Run("sends multipart with chat and file", func(t *testing.T) {
		var gotPath, gotChat string
		var gotPhoto []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			require.NoError(t, r.ParseMultipartForm(1<<20))
			gotChat = r.FormValue("chat_id")
			f, _, err := r.FormFile("photo")
			require.NoError(t, err)
			gotPhoto, _ = io.ReadAll(f)
		}))
		defer srv.Close()

		c := NewPhotoClient(srv.Client(), "TOKEN")
		c.baseURL = srv.URL
		require.NoError(t, c.SendPhoto(t.Context(), "42", []byte("png-bytes")))

		assert.Equal(t, "/botTOKEN/sendPhoto", gotPath)
		assert.Equal(t, "42", gotChat)
		assert.Equal(t, []byte("png-bytes"), gotPhoto)
	})

	t.Run("maps status codes and runs middlewares", func(t *testing.T) {
		tests := []struct {
			status int
			want   error
		}{
			{http.StatusForbidden, tc.ErrForbidden},
			{http.StatusTooManyRequests, tc.ErrTooManyRequests},
		}
		for _, tt := range tests {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tt.status) }))

			var middlewareChat string
			mw := func(next tc.Handler) tc.Handler {
				return func(ctx tc.Context) error {
					middlewareChat, _ = ctx.ChatID()
					return next(ctx)
				}
			}
			c := NewPhotoClient(srv.Client(), "T", mw)
			c.baseURL = srv.URL

			err := c.SendPhoto(t.Context(), "7", []byte("x"))
			srv.Close()
			require.ErrorIs(t, err, tt.want)
			assert.Equal(t, "7", middlewareChat)
		}
	})

	t.Run("unexpected status is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
		defer srv.Close()
		c := NewPhotoClient(srv.Client(), "T")
		c.baseURL = srv.URL
		require.Error(t, c.SendPhoto(t.Context(), "7", []byte("x")))
	})
}

func TestPhotoClient_SendPhoto_ErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	c := NewPhotoClient(srv.Client(), "SECRET")
	c.baseURL = srv.URL
	srv.Close()

	err := c.SendPhoto(t.Context(), "7", []byte("x"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "SECRET")
}
