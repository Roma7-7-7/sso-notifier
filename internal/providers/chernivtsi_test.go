package providers

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
)

//go:embed testdata/chernivtsi_without_next_day.html
var chernivtsiWithoutNextDay []byte

//go:embed testdata/chernivtsi_with_next_day.html
var chernivtsiWithNextDay []byte

func TestChernivtsiProvider_Shutdowns(t *testing.T) {
	type fields struct {
		loadPage func(context.Context, string) ([]byte, error)
	}
	tests := []struct {
		name            string
		fields          fields
		want            dal.Shutdowns
		wantHasNextPage bool
		wantErr         assert.ErrorAssertionFunc
	}{
		{
			name: "success_no_next_page",
			fields: fields{
				loadPage: func(_ context.Context, _ string) ([]byte, error) {
					return chernivtsiWithoutNextDay, nil
				},
			},
			want: testutil.NewShutdowns().WithDate("31.10.2025").
				WithGroup(1, "YYYYYYYYYYYYYYYYYYMNNNMYYYYYYYYYMNNNMYYYYYYYYYYY").
				WithGroup(2, "NNNMYYYYYYYYYYYYYYMNMYYYYYYYYYYYMNNNMYYYYYYYYYYY").
				WithGroup(3, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYY").
				WithGroup(4, "YYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNMYYYYYYYMNNNNNMYY").
				WithGroup(5, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYY").
				WithGroup(6, "YYYYMNNNNNMYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMN").
				WithGroup(7, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNNMYYYYYYYYY").
				WithGroup(8, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNNMYYYYYYYYY").
				WithGroup(9, "YYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYMNNNNNMYYYYYYYYY").
				WithGroup(10, "YYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYYYYYYYYMNNNNMYYY").
				WithGroup(11, "YYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNMYYYYYYYMNNNNMYYY").
				WithGroup(12, "YYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNMYYYYYYYYYYYYYYMN").
				Build(),
			wantHasNextPage: false,
			wantErr:         assert.NoError,
		},
		{
			name: "success_has_next_page",
			fields: fields{
				loadPage: func(_ context.Context, _ string) ([]byte, error) {
					return chernivtsiWithNextDay, nil
				},
			},
			want: testutil.NewShutdowns().WithDate("05.11.2025").
				WithGroup(1, "YYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYYYYY").
				WithGroup(2, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY").
				WithGroup(3, "YYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYMNNNNMYYYYY").
				WithGroup(4, "YYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYMNNNNMYYYYY").
				WithGroup(5, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNMYYYYY").
				WithGroup(6, "YYYYYYYYYYYYYYNMYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY").
				WithGroup(7, "YYYYYYYYYYYYYYNMYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY").
				WithGroup(8, "YYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYYYYYYYYY").
				WithGroup(9, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYNNNNMYYYYYYYYYYY").
				WithGroup(10, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYNNNNMYYYYYYYYYYY").
				WithGroup(11, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYY").
				WithGroup(12, "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYMNNNNNMYYYYYYYYYYY").
				Build(),
			wantHasNextPage: true,
			wantErr:         assert.NoError,
		},
		{
			name: "error_not_html",
			fields: fields{
				loadPage: func(_ context.Context, _ string) ([]byte, error) {
					return []byte("random text"), nil
				},
			},
			wantErr: assert.Error,
		},
		{
			name: "error_load_page",
			fields: fields{
				loadPage: func(_ context.Context, _ string) ([]byte, error) {
					return nil, assert.AnError
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Error(t, err, i...) && assert.ErrorIs(t, err, assert.AnError) && assert.ErrorContains(t, err, "load shutdowns page: ")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ChernivtsiProvider{
				loadPage: tt.fields.loadPage,
			}
			got, gotHasNextPage, err := p.Shutdowns(t.Context())

			if !tt.wantErr(t, err, "ChernivtsiProvider_Shutdowns()") {
				return
			}

			assert.Equalf(t, tt.want, got, "ChernivtsiProvider_Shutdowns()")
			assert.Equalf(t, tt.wantHasNextPage, gotHasNextPage, "ChernivtsiProvider_Shutdowns()")
		})
	}
}
