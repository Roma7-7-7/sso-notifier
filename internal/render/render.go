// Package render draws a one-day power schedule of a single group as a PNG hour grid.
package render

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	minutesInHour = 60
	minutesInDay  = 24 * minutesInHour
)

type Kind int

const (
	KindOn Kind = iota
	KindOff
	KindMaybe
)

type (
	// Segment is a half-open interval [FromMin, ToMin) in minutes since midnight. ToMin may be 1440.
	Segment struct {
		FromMin int
		ToMin   int
		Kind    Kind
	}

	Day struct {
		Title    string
		Subtitle string
		Segments []Segment
	}
)

const (
	width      = 720
	margin     = 30
	cols       = 6
	rows       = 4
	gap        = 5
	cellW      = (width - 2*margin - (cols-1)*gap) / cols
	cellH      = 90
	gridTop    = 115
	legendGap  = 25
	legendSize = 22
	bottomPad  = 40
)

//nolint:gochecknoglobals // palette is constant
var (
	colorBackground = color.RGBA{0x17, 0x21, 0x2b, 0xff}
	colorText       = color.RGBA{0xff, 0xff, 0xff, 0xff}
	colorMuted      = color.RGBA{0x8b, 0x9a, 0xa8, 0xff}
	colorLegend     = color.RGBA{0xcf, 0xd8, 0xe0, 0xff}
	colorShadow     = color.RGBA{0, 0, 0, 0xb0}
	colorUnknown    = color.RGBA{0x3a, 0x47, 0x54, 0xff}
	kindColors      = map[Kind]color.RGBA{
		KindOn:    {0x2e, 0x9e, 0x5b, 0xff},
		KindOff:   {0xd6, 0x45, 0x45, 0xff},
		KindMaybe: {0xe8, 0xb9, 0x23, 0xff},
	}
	kindLabels = []struct {
		kind  Kind
		label string
	}{
		{KindOn, "Є світло"},
		{KindOff, "Відключення"},
		{KindMaybe, "Можливо"},
	}
)

type fonts struct {
	title, subtitle, hour, legend font.Face
}

//nolint:gochecknoglobals // fonts are parsed once
var (
	fontsOnce   sync.Once
	loadedFonts fonts
	fontsErr    error
)

func loadFonts() (fonts, error) {
	fontsOnce.Do(func() {
		regular, err := opentype.Parse(goregular.TTF)
		if err != nil {
			fontsErr = fmt.Errorf("parse regular font: %w", err)
			return
		}
		bold, err := opentype.Parse(gobold.TTF)
		if err != nil {
			fontsErr = fmt.Errorf("parse bold font: %w", err)
			return
		}
		face := func(f *opentype.Font, size float64) font.Face {
			if fontsErr != nil {
				return nil
			}
			face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
			if err != nil {
				fontsErr = fmt.Errorf("create font face: %w", err)
			}
			return face
		}
		loadedFonts = fonts{
			title:    face(bold, 28),
			subtitle: face(regular, 18),
			hour:     face(bold, 26),
			legend:   face(regular, 18),
		}
	})
	return loadedFonts, fontsErr
}

// Render returns PNG bytes. Every hour is a cell filled with segments clipped to that hour, proportional by minute,
// so any granularity (30m, 15m, ...) is drawn exactly.
func Render(d Day) ([]byte, error) {
	if err := validate(d); err != nil {
		return nil, fmt.Errorf("validate day: %w", err)
	}
	f, err := loadFonts()
	if err != nil {
		return nil, fmt.Errorf("load fonts: %w", err)
	}

	legendY := gridTop + rows*(cellH+gap) + legendGap
	img := image.NewRGBA(image.Rect(0, 0, width, legendY+legendSize+bottomPad))
	draw.Draw(img, img.Bounds(), image.NewUniform(colorBackground), image.Point{}, draw.Src)

	drawText(img, f.title, margin, 50, d.Title, colorText, false)
	drawText(img, f.subtitle, margin, 82, d.Subtitle, colorMuted, false)

	for h := range 24 {
		x := margin + (h%cols)*(cellW+gap)
		y := gridTop + (h/cols)*(cellH+gap)
		drawHour(img, f, d.Segments, h, x, y)
	}

	x := 60
	for _, kl := range kindLabels {
		fillRect(img, x, legendY, legendSize, legendSize, kindColors[kl.kind])
		drawText(img, f.legend, x+legendSize+10, legendY+legendSize-4, kl.label, colorLegend, false)
		x += 210 //nolint:mnd // legend column step
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}

func validate(d Day) error {
	if len(d.Segments) == 0 {
		return errors.New("no segments")
	}
	for _, s := range d.Segments {
		if s.FromMin < 0 || s.ToMin > minutesInDay || s.FromMin >= s.ToMin {
			return fmt.Errorf("invalid segment %d-%d", s.FromMin, s.ToMin)
		}
		if _, ok := kindColors[s.Kind]; !ok {
			return fmt.Errorf("unknown kind %d", s.Kind)
		}
	}
	return nil
}

func drawHour(img *image.RGBA, f fonts, segments []Segment, hour, x, y int) {
	hourStart := hour * minutesInHour
	fillRect(img, x, y, cellW, cellH, colorUnknown)
	for _, s := range segments {
		from := max(s.FromMin, hourStart)
		to := min(s.ToMin, hourStart+minutesInHour)
		if to <= from {
			continue
		}
		x1 := x + (from-hourStart)*cellW/minutesInHour
		x2 := x + (to-hourStart)*cellW/minutesInHour
		fillRect(img, x1, y, x2-x1, cellH, kindColors[s.Kind])
	}

	label := fmt.Sprintf("%02d", hour)
	w := textWidth(f.hour, label)
	drawText(img, f.hour, x+(cellW-w)/2, y+cellH/2+9, label, colorText, true) //nolint:mnd // vertical centering
}

func fillRect(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, draw.Src)
}

func textWidth(face font.Face, s string) int {
	return (&font.Drawer{Face: face}).MeasureString(s).Round()
}

func drawText(img *image.RGBA, face font.Face, x, y int, s string, c color.RGBA, shadow bool) {
	if shadow {
		for _, off := range [][2]int{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {0, 2}} {
			d := &font.Drawer{Dst: img, Src: image.NewUniform(colorShadow), Face: face, Dot: fixed.P(x+off[0], y+off[1])}
			d.DrawString(s)
		}
	}
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
}
