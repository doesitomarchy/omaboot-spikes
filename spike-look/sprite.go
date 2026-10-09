package main

// Boot (the "Sprite" draft from the prototype) and the textmode-style animation field.

import (
	"math"
	"strings"
)

var pix = map[byte]int{'k': K, 'r': RED, 'g': GRN, 'y': AMB, 'b': BLU, 'm': VIO, 'c': CYN, 'w': GRY,
	'K': DIM, 'R': BRED, 'G': BGRN, 'Y': YEL, 'B': BBLU, 'M': BVIO, 'C': BCYN, 'W': WHT}

// pixels draws one char per pixel, two pixel rows per text row (▀ ▄ █). Backgrounds stay 0–7,
// so a cell never holds two bright colours (the VT rule found in the prototype).
func pixels(x, y int, rows []string) string {
	var b strings.Builder
	for r := 0; r < len(rows); r += 2 {
		top, bot := rows[r], ""
		if r+1 < len(rows) {
			bot = rows[r+1]
		}
		w := max(len(top), len(bot))
		for i := 0; i < w; i++ {
			a, bb := -1, -1
			if i < len(top) {
				if v, ok := pix[top[i]]; ok {
					a = v
				}
			}
			if i < len(bot) {
				if v, ok := pix[bot[i]]; ok {
					bb = v
				}
			}
			if a < 0 && bb < 0 {
				continue
			}
			b.WriteString(at(x+i, y+r/2))
			if a == bb {
				b.WriteString(col(a, -1) + "█")
				continue
			}
			if a < 0 {
				a = K
			}
			if bb < 0 {
				bb = K
			}
			if a > 7 && bb > 7 {
				bb -= 8
			}
			if a > 7 || bb <= 7 {
				b.WriteString(col(a, bb) + "▀")
			} else {
				b.WriteString(col(bb, a) + "▄")
			}
		}
	}
	return b.String() + reset
}

// bootRows gives Boot's pixel rows at time t (seconds): idle life only (blink, glance, bob).
func bootRows(t float64, mood string) []string {
	k := math.Floor(t / 3.9)
	ph := t - k*3.9
	blink := math.Abs(ph-(1.2+hash(k, 7)*1.4)) < 0.09
	g := hash(math.Floor(t/2.7), 3)
	glance := 0
	if g < 0.2 {
		glance = -1
	} else if g > 0.8 {
		glance = 1
	}
	tip := "y"
	if int(t*2)%2 == 1 {
		tip = "Y"
	}
	face := func() []byte { return []byte(".ckkkkkkkkkkkkc.") }
	r4, r5, r6, r7, r8 := face(), face(), face(), face(), face()
	L, R := 4+glance, 9+glance
	switch {
	case blink:
		for _, p := range []int{L, R} {
			for i := 0; i < 3; i++ {
				r5[p+i] = 'C'
			}
		}
	case mood == "pleased":
		for _, p := range []int{L, R} {
			r5[p], r4[p+1], r5[p+2] = 'W', 'W', 'W'
		}
	default:
		for _, p := range []int{L, R} {
			for i := 0; i < 3; i++ {
				r4[p+i], r5[p+i] = 'W', 'W'
			}
		}
	}
	M := 6 + glance
	if mood == "pleased" {
		r7[M], r8[M+1], r8[M+2], r7[M+3] = 'C', 'C', 'C', 'C'
	} else {
		r7[M+1], r7[M+2] = 'C', 'C'
	}
	rows := []string{
		".......{tip}{tip}.......", "........c.......", "..cccccccccccc..", ".ckkkkkkkkkkkkc.",
		string(r4), string(r5), string(r6), string(r7), string(r8),
		".ckkkkkkkkkkkkc.", "..cccccccccccc..", "....cc....cc....",
	}
	rows[0] = strings.ReplaceAll(rows[0], "{tip}", tip)
	if int(t)%2 == 1 {
		rows = append([]string{"................"}, rows...)
	} else {
		rows = append(rows, "................")
	}
	return rows
}

func hash(a, b float64) float64 {
	s := math.Sin(a*127.1+b*311.7) * 43758.5453
	return s - math.Floor(s)
}

// field writes one frame of a textmode.js-style band: per-cell maths → character ramp → colour.
// Every cell changes every frame, so it is the worst case for the console.
var ramp = []rune(" .:-=+*#%@")
var rampCol = []int{K, DIM, BLU, BLU, VIO, VIO, BVIO, CYN, BCYN, WHT}

func field(b *strings.Builder, x0, y0, w, h int, t float64) {
	last := -1
	for y := 0; y < h; y++ {
		b.WriteString(at(x0, y0+y))
		for x := 0; x < w; x++ {
			fx, fy := float64(x), float64(y)*2
			v := math.Sin(fx*0.11+t*1.7) + math.Sin(fy*0.23-t*1.1) + math.Sin((fx+fy)*0.07+t*0.6) +
				math.Sin(math.Hypot(fx-float64(w)/2, fy-float64(h))*0.15-t*2.2)
			i := int((v + 4) / 8 * float64(len(ramp)))
			i = min(max(i, 0), len(ramp)-1)
			if rampCol[i] != last {
				b.WriteString(col(rampCol[i], K))
				last = rampCol[i]
			}
			b.WriteRune(ramp[i])
		}
	}
	b.WriteString(reset)
}

func sin(x float64) float64 { return math.Sin(x) }
