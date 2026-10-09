package profileimage

import (
	"bytes"
	"encoding/binary"
)

func readOrientation(data []byte, format string) int {
	switch format {
	case "jpeg":
		for p := 2; p+4 <= len(data); {
			if data[p] != 0xff {
				break
			}
			marker := data[p+1]
			if marker == 0xda || marker == 0xd9 {
				break
			}
			n := int(binary.BigEndian.Uint16(data[p+2 : p+4]))
			if n < 2 || n > len(data)-p-2 {
				break
			}
			block := data[p+4 : p+2+n]
			if marker == 0xe1 && bytes.HasPrefix(block, []byte("Exif\x00\x00")) {
				return tiffOrientation(block[6:])
			}
			p += n + 2
		}
	case "png":
		for p := 8; p+12 <= len(data); {
			n := uint64(binary.BigEndian.Uint32(data[p : p+4]))
			if n > uint64(len(data)-p-12) {
				break
			}
			end := p + 8 + int(n)
			if string(data[p+4:p+8]) == "eXIf" {
				return tiffOrientation(data[p+8 : end])
			}
			p = end + 4
		}
	case "webp":
		for p := 12; p+8 <= len(data); {
			n := uint64(binary.LittleEndian.Uint32(data[p+4 : p+8]))
			if n > uint64(len(data)-p-8) {
				break
			}
			end := p + 8 + int(n)
			if string(data[p:p+4]) == "EXIF" {
				block := data[p+8 : end]
				if bytes.HasPrefix(block, []byte("Exif\x00\x00")) {
					block = block[6:]
				}
				return tiffOrientation(block)
			}
			p = end + int(n%2)
		}
	}
	return 1
}
func tiffOrientation(data []byte) int {
	if len(data) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1
	}
	offset := uint64(order.Uint32(data[4:8]))
	if offset > uint64(len(data)-2) {
		return 1
	}
	p := int(offset)
	count := int(order.Uint16(data[p : p+2]))
	p += 2
	if count > (len(data)-p)/12 {
		return 1
	}
	for i := 0; i < count; i++ {
		field := data[p+i*12 : p+(i+1)*12]
		if order.Uint16(field[:2]) == 0x112 && order.Uint16(field[2:4]) == 3 && order.Uint32(field[4:8]) == 1 {
			n := int(order.Uint16(field[8:10]))
			if n >= 1 && n <= 8 {
				return n
			}
		}
	}
	return 1
}
