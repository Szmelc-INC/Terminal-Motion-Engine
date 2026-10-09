package app

import (
	"math"

	"github.com/Szmelc-INC/Terminal-Motion-Engine/internal/engine"
)

// Geometry is where a picture lands on a cell grid and how it has to be
// decoded to get there.
type Geometry struct {
	Cols, Rows   int     // cells covered by the picture
	W, H         int     // pixels to decode (cells × sub-cells of the mode)
	CropX, CropY float64 // fraction of the source kept (fill mode, zoom)
}

// Fit places a picture with the given display aspect ratio on a cols×rows
// grid whose cells are cellAspect wide for every unit of height.
func Fit(aspect float64, l engine.Look, cols, rows int, cellAspect float64) Geometry {
	return FitZoom(aspect, l, cols, rows, cellAspect, 1)
}

// FitZoom is Fit with the picture scaled by zoom. Below 1 the picture
// shrinks inside the grid. Above 1 it grows until it covers the grid, and
// from there on the source is cropped instead.
func FitZoom(aspect float64, l engine.Look, cols, rows int, cellAspect, zoom float64) Geometry {
	g := Geometry{CropX: 1, CropY: 1}
	fc, fr := float64(cols), float64(rows)
	viewAR := fc * cellAspect / fr
	switch l.Fit {
	case "stretch":
	case "fill":
		if aspect > viewAR {
			g.CropX = viewAR / aspect
		} else {
			g.CropY = aspect / viewAR
		}
	default:
		if aspect > viewAR {
			fr = fc * cellAspect / aspect
		} else {
			fc = fr * aspect / cellAspect
		}
	}
	if zoom <= 0 {
		zoom = 1
	}
	fc, fr = fc*zoom, fr*zoom
	if fc > float64(cols) {
		g.CropX *= float64(cols) / fc
		fc = float64(cols)
	}
	if fr > float64(rows) {
		g.CropY *= float64(rows) / fr
		fr = float64(rows)
	}
	g.Cols = max(1, min(cols, int(math.Round(fc))))
	g.Rows = max(1, min(rows, int(math.Round(fr))))
	sx, sy := engine.SubCells(l.Mode)
	g.W, g.H = g.Cols*sx, g.Rows*sy
	return g
}
