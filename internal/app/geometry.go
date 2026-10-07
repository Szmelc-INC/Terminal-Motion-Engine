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
	CropX, CropY float64 // fraction of the source kept (fill mode)
}

// Fit places a picture with the given display aspect ratio on a cols×rows
// grid whose cells are cellAspect wide for every unit of height.
func Fit(aspect float64, l engine.Look, cols, rows int, cellAspect float64) Geometry {
	g := Geometry{Cols: cols, Rows: rows, CropX: 1, CropY: 1}
	viewAR := float64(cols) * cellAspect / float64(rows)
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
			g.Rows = int(math.Round(float64(cols) * cellAspect / aspect))
		} else {
			g.Cols = int(math.Round(float64(rows) * aspect / cellAspect))
		}
	}
	g.Cols = max(1, min(cols, g.Cols))
	g.Rows = max(1, min(rows, g.Rows))
	sx, sy := engine.SubCells(l.Mode)
	g.W, g.H = g.Cols*sx, g.Rows*sy
	return g
}
