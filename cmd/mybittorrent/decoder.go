package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Decoder struct {
	raw          string
	decoderFuncs []DecoderFunc
}

type DecoderFunc func(string) (any, int, error)

func NewDecoder(raw string) *Decoder {
	d := &Decoder{raw: raw}
	return d.WithDecoderFunc(d.decodeSlice).WithDecoderFunc(d.decodeMap).WithDecoderFunc(d.decodeString).WithDecoderFunc(d.decodeInteger)
}

func (d *Decoder) WithDecoderFunc(f DecoderFunc) *Decoder {
	d.decoderFuncs = append(d.decoderFuncs, f)
	return d
}

func (d *Decoder) Decode() (any, error) {
	for _, f := range d.decoderFuncs {
		if r, l, err := f(d.raw); l > 0 || err != nil {
			return r, nil
		}
	}

	return nil, fmt.Errorf("failed to decode value: %s", d.raw)
}

func (d *Decoder) decodeString(chunk string) (any, int, error) {
	if !unicode.IsDigit(rune(chunk[0])) {
		return "", 0, nil
	}

	var firstColonIndex int

	for i := 0; i < len(chunk); i++ {
		if chunk[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := chunk[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, err
	}

	val := chunk[firstColonIndex+1 : firstColonIndex+1+length]

	return val, length + 1 + len(lengthStr), nil
}

func (d *Decoder) decodeInteger(chunk string) (any, int, error) {
	if rune(chunk[0]) != 'i' {
		return 0, 0, nil
	}

	num, err := strconv.Atoi(chunk[1:strings.IndexByte(chunk, 'e')])
	if err != nil {
		return 0, 0, err
	}

	return num, len(strconv.Itoa(num)) + 2, nil
}

func (d *Decoder) decodeSlice(chunk string) (any, int, error) {
	if rune(chunk[0]) != 'l' {
		return nil, 0, nil
	}

	if rune(chunk[1]) == 'e' {
		return []any{}, 2, nil
	}

	result := make([]any, 0, 32)
	offset := 1

	for rune(chunk[offset]) != 'e' {
		for _, f := range d.decoderFuncs {
			if v, l, err := f(chunk[offset:]); l > 0 {
				if err != nil {
					return nil, 0, err
				}
				result = append(result, v)
				offset += l
				break
			}
		}
	}

	return result, offset + 1, nil
}

func (d *Decoder) decodeMap(chunk string) (any, int, error) {
	if rune(chunk[0]) != 'd' {
		return nil, 0, nil
	}

	if rune(chunk[1]) == 'e' {
		return map[string]any{}, 2, nil
	}

	result := make(map[string]any, 32)
	offset := 1

	for rune(chunk[offset]) != 'e' {
		key, keyLen, err := d.decodeString(chunk[offset:])
		if err != nil {
			return nil, 0, err
		}

		if keyLen == 0 {
			return nil, 0, fmt.Errorf("invalid bencoded map key length")
		}

		for _, f := range d.decoderFuncs {
			if v, l, err := f(chunk[offset+keyLen:]); l > 0 {
				if err != nil {
					return nil, 0, err
				}

				result[key.(string)] = v
				offset += keyLen + l
				break
			}
		}
	}

	return result, offset + 1, nil
}
