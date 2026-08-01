package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

type romfsEntry struct {
	path, name                             string
	next, parent, child, dataAddress, size uint32
	checksum                               uint32
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gifscan <disk.img> <output-dir>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	if len(data) < 0x44 || data[0] != 0x10 || data[1] != 0 || data[2] != 0 {
		if len(data) <= 0x80600 {
			panic("input has no firmware payload")
		}
		data = data[0x80600:]
		for i := 0; i+1 < len(data); i += 2 {
			data[i], data[i+1] = data[i+1], data[i]
		}
	}
	romLen := int((binary.BigEndian.Uint32(data[0x0C:])-3)*4 + 0x0C)
	if romLen <= 0 || romLen > len(data) {
		panic("invalid firmware length")
	}
	data = data[:romLen]
	if err := os.MkdirAll(os.Args[2], 0755); err != nil {
		panic(err)
	}
	manifest, err := os.Create(filepath.Join(os.Args[2], "resources.txt"))
	if err != nil {
		panic(err)
	}
	buildAddress := binary.BigEndian.Uint32(data[0x30:])
	romfsAddress := binary.BigEndian.Uint32(data[0x24:])
	entries, err := walkROMFS(data, buildAddress, romfsAddress)
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		compression := "none"
		if entry.size&0x90000000 == 0x90000000 {
			compression = "lzss"
		} else if entry.size&0x80000000 != 0 {
			compression = "lzpf"
		}
		storedSize := entry.size & 0x00FFFFFF
		fmt.Fprintf(manifest, "ROMFS path=%s address=0x%08X size=%d compression=%s data=0x%08X\n", entry.path, entry.dataAddress, storedSize, compression, entry.dataAddress-buildAddress)
		if strings.EqualFold(entry.path, "/ROM/Cache/WebTVLogoJewel.gif") || strings.EqualFold(entry.path, "/ROM/Animations/WebTVLogo.cel") {
			off := int(entry.dataAddress - buildAddress)
			if off < 0 || off+int(storedSize) > len(data) {
				panic(entry.path + " is outside firmware")
			}
			if err := os.WriteFile(filepath.Join(os.Args[2], filepath.Base(entry.path)+".raw"), data[off:off+int(storedSize)], 0644); err != nil {
				panic(err)
			}
		}
	}
	for off := 0; off+4 <= len(data); off++ {
		if bytes.Equal(data[off:off+4], []byte("BMIR")) {
			start, end := max(0, off-32), min(len(data), off+96)
			fmt.Fprintf(manifest, "BMIR offset=0x%08X context=%X\n", off, data[start:end])
		}
	}
	for off := 0; off+7 <= len(data); off++ {
		if !bytes.Equal(data[off:off+7], []byte("file://")) {
			continue
		}
		end := off
		for end < len(data) && data[end] >= 0x20 && data[end] <= 0x7E {
			end++
		}
		fmt.Fprintf(manifest, "URI offset=0x%08X %s\n", off, data[off:end])
		off = end
	}
	if err := manifest.Close(); err != nil {
		panic(err)
	}
	type asset struct {
		offset int
		image  image.Image
	}
	assets := make([]asset, 0)
	for off := 0; off+6 <= len(data); off++ {
		if !bytes.Equal(data[off:off+6], []byte("GIF87a")) && !bytes.Equal(data[off:off+6], []byte("GIF89a")) {
			continue
		}
		decoded, err := gif.Decode(bytes.NewReader(data[off:]))
		if err != nil {
			continue
		}
		path := filepath.Join(os.Args[2], fmt.Sprintf("gif_%03d_%08X.png", len(assets), off))
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		err = png.Encode(f, decoded)
		closeErr := f.Close()
		if err != nil {
			panic(err)
		}
		if closeErr != nil {
			panic(closeErr)
		}
		fmt.Printf("%03d offset=0x%08X size=%dx%d %s\n", len(assets), off, decoded.Bounds().Dx(), decoded.Bounds().Dy(), path)
		assets = append(assets, asset{offset: off, image: decoded})
	}
	const tileW, tileH, cols = 160, 120, 8
	rows := (len(assets) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, tileW*cols, tileH*rows))
	draw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: color.RGBA{24, 24, 24, 255}}, image.Point{}, draw.Src)
	for i, asset := range assets {
		dst := image.Rect((i%cols)*tileW, (i/cols)*tileH, (i%cols+1)*tileW, (i/cols+1)*tileH)
		scaleFit(sheet, dst, asset.image)
	}
	f, err := os.Create(filepath.Join(os.Args[2], "contact-sheet.png"))
	if err != nil {
		panic(err)
	}
	err = png.Encode(f, sheet)
	closeErr := f.Close()
	if err != nil {
		panic(err)
	}
	if closeErr != nil {
		panic(closeErr)
	}
}

func walkROMFS(data []byte, buildAddress, romfsAddress uint32) ([]romfsEntry, error) {
	entries := make([]romfsEntry, 0)
	visited := make(map[uint32]bool)
	var walk func(uint32, uint32, string, bool) error
	walk = func(address, parent uint32, path string, base bool) error {
		for address != 0 {
			if visited[address] {
				return fmt.Errorf("ROMFS node cycle at 0x%08X", address)
			}
			visited[address] = true
			if address < buildAddress || uint64(address-buildAddress)+56 > uint64(len(data)) {
				return fmt.Errorf("ROMFS node 0x%08X is outside firmware", address)
			}
			off := int(address - buildAddress)
			node := data[off : off+56]
			nameBytes := node[28:56]
			if end := bytes.IndexByte(nameBytes, 0); end >= 0 {
				nameBytes = nameBytes[:end]
			}
			entry := romfsEntry{
				path:        path + "/" + string(nameBytes),
				name:        string(nameBytes),
				next:        binary.BigEndian.Uint32(node[0:]),
				parent:      binary.BigEndian.Uint32(node[4:]),
				child:       binary.BigEndian.Uint32(node[8:]),
				dataAddress: binary.BigEndian.Uint32(node[12:]),
				size:        binary.BigEndian.Uint32(node[16:]),
				checksum:    binary.BigEndian.Uint32(node[24:]),
			}
			if entry.parent == parent {
				entries = append(entries, entry)
				if entry.child != 0 {
					if err := walk(entry.child, address, entry.path, false); err != nil {
						return err
					}
				}
			}
			if base {
				break
			}
			address = entry.next
		}
		return nil
	}
	if romfsAddress < buildAddress+64 {
		return nil, fmt.Errorf("invalid ROMFS address 0x%08X", romfsAddress)
	}
	if err := walk(romfsAddress-64, 0, "", true); err != nil {
		return nil, err
	}
	return entries, nil
}

func scaleFit(dst *image.RGBA, tile image.Rectangle, src image.Image) {
	sb := src.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if w == 0 || h == 0 {
		return
	}
	scale := min(float64(tile.Dx())/float64(w), float64(tile.Dy())/float64(h))
	dw, dh := max(1, int(float64(w)*scale)), max(1, int(float64(h)*scale))
	x0, y0 := tile.Min.X+(tile.Dx()-dw)/2, tile.Min.Y+(tile.Dy()-dh)/2
	for y := 0; y < dh; y++ {
		sy := sb.Min.Y + y*h/dh
		for x := 0; x < dw; x++ {
			sx := sb.Min.X + x*w/dw
			dst.Set(x0+x, y0+y, src.At(sx, sy))
		}
	}
}
