package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"fyne.io/fyne/v2"
)

// Ícones desenhados em código (sem arquivos externos).

var (
	colGray  = color.NRGBA{0x8a, 0x8f, 0x98, 0xff}
	colAmber = color.NRGBA{0xf2, 0xa9, 0x00, 0xff}
	colGreen = color.NRGBA{0x1f, 0xb3, 0x6b, 0xff}
	colRed   = color.NRGBA{0xe0, 0x3e, 0x3e, 0xff}
	colTeal  = color.NRGBA{0x0f, 0x6e, 0x6e, 0xff}
)

// coverage calcula quanto do pixel (x,y) está dentro da forma (4x4 amostras).
func coverage(x, y int, inside func(fx, fy float64) bool) float64 {
	n := 0
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if inside(float64(x)+(float64(i)+0.5)/4, float64(y)+(float64(j)+0.5)/4) {
				n++
			}
		}
	}
	return float64(n) / 16
}

func blend(dst color.NRGBA, src color.NRGBA, a float64) color.NRGBA {
	if a <= 0 {
		return dst
	}
	sa := float64(src.A) / 255 * a
	da := float64(dst.A) / 255
	oa := sa + da*(1-sa)
	if oa == 0 {
		return color.NRGBA{}
	}
	mix := func(s, d uint8) uint8 {
		return uint8(math.Round((float64(s)*sa + float64(d)*da*(1-sa)) / oa))
	}
	return color.NRGBA{mix(src.R, dst.R), mix(src.G, dst.G), mix(src.B, dst.B), uint8(math.Round(oa * 255))}
}

type layer struct {
	col    color.NRGBA
	inside func(fx, fy float64) bool
}

func render(size int, layers ...layer) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := color.NRGBA{}
			for _, l := range layers {
				px = blend(px, l.col, coverage(x, y, l.inside))
			}
			img.SetNRGBA(x, y, px)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func circle(cx, cy, r float64) func(float64, float64) bool {
	return func(x, y float64) bool { return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r }
}

func ring(cx, cy, r1, r2 float64) func(float64, float64) bool {
	return func(x, y float64) bool {
		d := (x-cx)*(x-cx) + (y-cy)*(y-cy)
		return d <= r2*r2 && d >= r1*r1
	}
}

func roundedSquare(s, r float64) func(float64, float64) bool {
	return func(x, y float64) bool {
		cx := math.Max(r, math.Min(x, s-r))
		cy := math.Max(r, math.Min(y, s-r))
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}
}

// Ícone do app: quadrado arredondado com um "sinal" saindo de um ponto.
func appIconPNG(size int) []byte {
	s := float64(size)
	white := color.NRGBA{0xff, 0xff, 0xff, 0xff}
	cx, cy := s*0.5, s*0.58
	return render(size,
		layer{colTeal, roundedSquare(s, s*0.22)},
		layer{white, circle(cx, cy, s*0.09)},
		layer{white, arc(cx, cy, s*0.19, s*0.25)},
		layer{white, arc(cx, cy, s*0.31, s*0.37)},
	)
}

// arc é um anel limitado a um setor de 90° apontado para cima.
func arc(cx, cy, r1, r2 float64) func(float64, float64) bool {
	in := ring(cx, cy, r1, r2)
	return func(x, y float64) bool {
		if !in(x, y) {
			return false
		}
		ang := math.Atan2(cy-y, x-cx) // 90° = para cima
		return ang >= math.Pi/4 && ang <= 3*math.Pi/4
	}
}

// Ícone da bandeja: o mesmo desenho, com um ponto de status no canto.
func trayIconPNG(status color.NRGBA) []byte {
	const size = 64
	s := float64(size)
	white := color.NRGBA{0xff, 0xff, 0xff, 0xff}
	cx, cy := s*0.5, s*0.58
	return render(size,
		layer{colTeal, roundedSquare(s, s*0.22)},
		layer{white, circle(cx, cy, s*0.09)},
		layer{white, arc(cx, cy, s*0.19, s*0.25)},
		layer{white, arc(cx, cy, s*0.31, s*0.37)},
		layer{white, circle(s*0.78, s*0.78, s*0.21)},
		layer{status, circle(s*0.78, s*0.78, s*0.16)},
	)
}

var (
	resAppIcon   = fyne.NewStaticResource("evoproxy.png", appIconPNG(256))
	resTrayGray  = fyne.NewStaticResource("tray-off.png", trayIconPNG(colGray))
	resTrayAmber = fyne.NewStaticResource("tray-wait.png", trayIconPNG(colAmber))
	resTrayGreen = fyne.NewStaticResource("tray-on.png", trayIconPNG(colGreen))
	resTrayRed   = fyne.NewStaticResource("tray-err.png", trayIconPNG(colRed))
)
