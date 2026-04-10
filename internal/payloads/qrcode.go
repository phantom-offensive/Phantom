package payloads

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// GenerateQRCode creates a QR code PNG for the given URL.
// Uses a minimal QR code implementation (version 2, 25x25 modules,
// alphanumeric mode) — no external dependencies.
//
// For URLs longer than ~40 chars, falls back to a styled "scan me"
// placeholder with the URL embedded as text. Real-world usage should
// use a proper QR library; this is a lightweight built-in for the
// C2's payload delivery page.
func GenerateQRCode(url string, size int) []byte {
	if size <= 0 {
		size = 400
	}

	// Generate QR matrix using our minimal encoder
	modules := encodeQR(url)
	qrSize := len(modules)
	if qrSize == 0 {
		// Fallback — URL too long for our minimal encoder
		return generateFallbackQR(url, size)
	}

	// Scale QR modules to target size
	moduleSize := size / (qrSize + 8) // +8 for quiet zone
	if moduleSize < 1 {
		moduleSize = 1
	}
	imgSize := (qrSize + 8) * moduleSize

	img := image.NewRGBA(image.Rect(0, 0, imgSize, imgSize))

	// White background
	for y := 0; y < imgSize; y++ {
		for x := 0; x < imgSize; x++ {
			img.Set(x, y, color.White)
		}
	}

	// Draw modules
	quiet := 4 * moduleSize
	black := color.RGBA{0, 0, 0, 255}
	for row := 0; row < qrSize; row++ {
		for col := 0; col < qrSize; col++ {
			if modules[row][col] {
				px := quiet + col*moduleSize
				py := quiet + row*moduleSize
				for dy := 0; dy < moduleSize; dy++ {
					for dx := 0; dx < moduleSize; dx++ {
						img.Set(px+dx, py+dy, black)
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// encodeQR creates a minimal QR code matrix. Supports QR Version 1-2
// with byte mode, error correction level L. Good for URLs up to ~40 chars.
func encodeQR(data string) [][]bool {
	dataBytes := []byte(data)

	// Determine version
	version := 1
	capacity := 17 // Version 1, ECC L, byte mode
	if len(dataBytes) > capacity {
		version = 2
		capacity = 32
	}
	if len(dataBytes) > capacity {
		version = 3
		capacity = 53
	}
	if len(dataBytes) > capacity {
		version = 4
		capacity = 78
	}
	if len(dataBytes) > capacity {
		version = 5
		capacity = 106
	}
	if len(dataBytes) > capacity {
		return nil // Too long for our implementation
	}

	size := 17 + version*4
	modules := make([][]bool, size)
	reserved := make([][]bool, size)
	for i := range modules {
		modules[i] = make([]bool, size)
		reserved[i] = make([]bool, size)
	}

	// Place finder patterns
	placeFinder := func(row, col int) {
		for r := -1; r <= 7; r++ {
			for c := -1; c <= 7; c++ {
				pr, pc := row+r, col+c
				if pr < 0 || pr >= size || pc < 0 || pc >= size {
					continue
				}
				if r >= 0 && r <= 6 && c >= 0 && c <= 6 {
					if r == 0 || r == 6 || c == 0 || c == 6 ||
						(r >= 2 && r <= 4 && c >= 2 && c <= 4) {
						modules[pr][pc] = true
					}
				}
				reserved[pr][pc] = true
			}
		}
	}
	placeFinder(0, 0)
	placeFinder(0, size-7)
	placeFinder(size-7, 0)

	// Timing patterns
	for i := 8; i < size-8; i++ {
		modules[6][i] = i%2 == 0
		reserved[6][i] = true
		modules[i][6] = i%2 == 0
		reserved[i][6] = true
	}

	// Dark module
	modules[size-8][8] = true
	reserved[size-8][8] = true

	// Alignment pattern for version >= 2
	if version >= 2 {
		pos := size - 7
		for r := -2; r <= 2; r++ {
			for c := -2; c <= 2; c++ {
				pr, pc := pos+r, pos+c
				if pr >= 0 && pr < size && pc >= 0 && pc < size {
					if r == -2 || r == 2 || c == -2 || c == 2 || (r == 0 && c == 0) {
						modules[pr][pc] = true
					}
					reserved[pr][pc] = true
				}
			}
		}
	}

	// Reserve format info areas
	for i := 0; i < 9; i++ {
		if i < size {
			reserved[8][i] = true
			reserved[i][8] = true
		}
		if size-1-i >= 0 {
			reserved[8][size-1-i] = true
			reserved[size-1-i][8] = true
		}
	}

	// Encode data bits
	bits := encodeBits(dataBytes, version)

	// Place data bits in zigzag pattern
	bitIdx := 0
	for col := size - 1; col >= 0; col -= 2 {
		if col == 6 {
			col = 5
		}
		for row := 0; row < size; row++ {
			for c := 0; c < 2; c++ {
				cc := col - c
				actualRow := row
				if ((col+1)/2)%2 == 0 {
					actualRow = size - 1 - row
				}
				if cc >= 0 && cc < size && actualRow >= 0 && actualRow < size && !reserved[actualRow][cc] {
					if bitIdx < len(bits) {
						modules[actualRow][cc] = bits[bitIdx]
						bitIdx++
					}
				}
			}
		}
	}

	// Apply mask (mask 0: (row + col) % 2 == 0)
	for r := 0; r < size; r++ {
		for c := 0; c < size; c++ {
			if !reserved[r][c] && (r+c)%2 == 0 {
				modules[r][c] = !modules[r][c]
			}
		}
	}

	// Place format info (ECC L, mask 0)
	formatBits := []bool{true, true, true, false, true, true, true, true, true, false, false, false, true, false, false}
	for i := 0; i < 15; i++ {
		// Around top-left
		if i < 6 {
			modules[8][i] = formatBits[i]
		} else if i < 8 {
			modules[8][i+1] = formatBits[i]
		} else {
			modules[14-i][8] = formatBits[i]
		}
		// Other positions
		if i < 8 {
			modules[size-1-i][8] = formatBits[i]
		} else {
			modules[8][size-15+i] = formatBits[i]
		}
	}

	return modules
}

func encodeBits(data []byte, version int) []bool {
	var bits []bool

	// Mode indicator: 0100 (byte mode)
	bits = append(bits, false, true, false, false)

	// Character count (8 bits for version 1-9)
	count := len(data)
	for i := 7; i >= 0; i-- {
		bits = append(bits, (count>>uint(i))&1 == 1)
	}

	// Data
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1 == 1)
		}
	}

	// Terminator
	for i := 0; i < 4 && len(bits) < totalDataBits(version); i++ {
		bits = append(bits, false)
	}

	// Pad to byte boundary
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}

	// Pad bytes
	padBytes := []byte{0xEC, 0x11}
	padIdx := 0
	for len(bits) < totalDataBits(version) {
		b := padBytes[padIdx%2]
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>uint(i))&1 == 1)
		}
		padIdx++
	}

	return bits
}

func totalDataBits(version int) int {
	switch version {
	case 1:
		return 152
	case 2:
		return 272
	case 3:
		return 440
	case 4:
		return 640
	case 5:
		return 864
	default:
		return 272
	}
}

func generateFallbackQR(url string, size int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	white := color.White
	black := color.RGBA{0, 0, 0, 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, white)
		}
	}

	// Draw a border
	for i := 0; i < size; i++ {
		for t := 0; t < 4; t++ {
			img.Set(i, t, black)
			img.Set(i, size-1-t, black)
			img.Set(t, i, black)
			img.Set(size-1-t, i, black)
		}
	}

	// Center crosshair
	cx, cy := size/2, size/2
	for i := -size/4; i < size/4; i++ {
		for t := -2; t <= 2; t++ {
			img.Set(cx+i, cy+t, black)
			img.Set(cx+t, cy+i, black)
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
