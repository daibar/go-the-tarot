package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"strings"
)

// asciiRamp runs from darkest to lightest, the same idea as jp2a's default ramp.
const asciiRamp = "  ...',;:clodxkO0KXNWM"

// renderCard turns a card's jpg into colored ASCII art, replacing the jp2a
// dependency. Reversed cards are flipped vertically, as `jp2a -y` did.
func renderCard(c Card, height int, color bool) (string, error) {
	f, err := assets.Open(fmt.Sprintf("assets/img/%d.jpg", c.ImageNumber()))
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	return renderImage(img, height, color, c.Reversed), nil
}

func renderImage(img image.Image, height int, color, flipY bool) string {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 || height <= 0 {
		return ""
	}
	// Terminal cells are roughly twice as tall as they are wide, so double the
	// width to keep the card's aspect ratio.
	width := int(float64(height) * 2 * float64(b.Dx()) / float64(b.Dy()))
	if width < 1 {
		width = 1
	}

	var sb strings.Builder
	for row := 0; row < height; row++ {
		srcRow := row
		if flipY {
			srcRow = height - 1 - row
		}
		y0 := b.Min.Y + srcRow*b.Dy()/height
		y1 := b.Min.Y + (srcRow+1)*b.Dy()/height
		for col := 0; col < width; col++ {
			x0 := b.Min.X + col*b.Dx()/width
			x1 := b.Min.X + (col+1)*b.Dx()/width
			r, g, bl := averageColor(img, x0, y0, x1, y1)
			lum := (299*r + 587*g + 114*bl) / 1000
			ch := asciiRamp[lum*(len(asciiRamp)-1)/255]
			if color {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm%c", r, g, bl, ch)
			} else {
				sb.WriteByte(ch)
			}
		}
		if color {
			sb.WriteString("\x1b[0m")
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// averageColor box-samples the pixels covering one character cell.
func averageColor(img image.Image, x0, y0, x1, y1 int) (int, int, int) {
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	var sumR, sumG, sumB, n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			sumR += int(r >> 8)
			sumG += int(g >> 8)
			sumB += int(b >> 8)
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return sumR / n, sumG / n, sumB / n
}
