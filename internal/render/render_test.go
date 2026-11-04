package render_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Roma7-7-7/sso-notifier/internal/render"
)

func decode(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	require.NoError(t, err)
	return img
}

func sameColor(t *testing.T, want, got color.Color, msg string) {
	t.Helper()
	wr, wg, wb, _ := want.RGBA()
	gr, gg, gb, _ := got.RGBA()
	assert.Equal(t, [3]uint32{wr, wg, wb}, [3]uint32{gr, gg, gb}, msg)
}

// cell geometry mirrors render.go: 6 columns, 105px wide cells, 5px gap, 30px margin, grid top 115
func cellPixel(hour, minute int) (int, int) {
	const cellW, gap, margin, gridTop, cellH = 105, 5, 30, 115, 90
	x := margin + (hour%6)*(cellW+gap) + minute*cellW/60
	y := gridTop + (hour/6)*(cellH+gap) + 3 // near top edge, away from the hour label
	return x, y
}

func TestRender_SegmentsPerMinute(t *testing.T) {
	green := color.RGBA{0x2e, 0x9e, 0x5b, 0xff}
	red := color.RGBA{0xd6, 0x45, 0x45, 0xff}
	yellow := color.RGBA{0xe8, 0xb9, 0x23, 0xff}

	b, err := render.Render(render.Day{
		Title:    "Група 4",
		Subtitle: "5 листопада",
		Segments: []render.Segment{
			{FromMin: 0, ToMin: 60, Kind: render.KindOn},
			{FromMin: 60, ToMin: 135, Kind: render.KindOff},    // ends mid-hour at :15
			{FromMin: 135, ToMin: 150, Kind: render.KindMaybe}, // 15-minute window
			{FromMin: 150, ToMin: 1440, Kind: render.KindOn},
		},
	})
	require.NoError(t, err)
	img := decode(t, b)

	at := func(h, m, offset int) color.Color {
		x, y := cellPixel(h, m)
		return img.At(x+offset, y)
	}

	sameColor(t, green, at(0, 0, 2), "00:00 on")
	sameColor(t, red, at(1, 0, 2), "01:00 off")
	sameColor(t, red, at(2, 0, 2), "02:00 off start")
	sameColor(t, red, at(2, 14, 0), "02:14 still off")
	sameColor(t, yellow, at(2, 16, 0), "02:16 maybe")
	sameColor(t, yellow, at(2, 29, 0), "02:29 maybe")
	sameColor(t, green, at(2, 31, 0), "02:31 on")
	sameColor(t, green, at(23, 30, 0), "23:30 on")
}

func TestRender_Errors(t *testing.T) {
	tests := map[string][]render.Segment{
		"empty":         nil,
		"inverted":      {{FromMin: 100, ToMin: 50, Kind: render.KindOn}},
		"past midnight": {{FromMin: 0, ToMin: 1441, Kind: render.KindOn}},
		"unknown kind":  {{FromMin: 0, ToMin: 60, Kind: render.Kind(42)}},
	}
	for name, segs := range tests {
		t.Run(name, func(t *testing.T) {
			b, err := render.Render(render.Day{Title: "t", Segments: segs})
			require.Error(t, err)
			assert.Nil(t, b)
		})
	}
}

// RENDER_OUT=/tmp/x.png go test ./internal/render -run Sample to inspect the result visually
func TestRender_Sample(t *testing.T) {
	out := os.Getenv("RENDER_OUT")
	if out == "" {
		t.Skip("RENDER_OUT not set")
	}
	b, err := render.Render(render.Day{
		Title:    "Група 4",
		Subtitle: "5 листопада · без світла 10г · можливо 2г",
		Segments: []render.Segment{
			{0, 60, render.KindOn}, {60, 180, render.KindOff}, {180, 210, render.KindMaybe}, {210, 420, render.KindOn},
			{420, 540, render.KindOff}, {540, 570, render.KindMaybe}, {570, 720, render.KindOn},
			{720, 900, render.KindOff}, {900, 930, render.KindMaybe}, {930, 1080, render.KindOn},
			{1080, 1260, render.KindOff}, {1260, 1275, render.KindMaybe}, {1275, 1440, render.KindOn},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(out, b, 0o600))
}
