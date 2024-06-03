package main

import (
	"bytes"
	"fmt"
	"io"
)

type File struct {
	Announce string   `bencode:"announce"`
	Info     FileInfo `bencode:"info"`
}

type FileInfo struct {
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
}

func NewFile() *File {
	return &File{}
}

func (f *File) ReadFrom(r io.ReadCloser) error {
	buf := new(bytes.Buffer)

	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}

	defer func(r io.ReadCloser) {
		err := r.Close()
		if err != nil {
			fmt.Printf("failed to close: %v+\n", err)
		}
	}(r)

	if err != nil {
		return err
	}

	decoded, err := NewDecoder(buf.String()).Decode()
	if err != nil {
		return err
	}

	content, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid contents")
	}

	f.Announce, ok = content["announce"].(string)
	if !ok {
		return fmt.Errorf("invalid announce field")
	}

	info, ok := content["info"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid info field")
	}

	f.Info.Length, ok = info["length"].(int)
	if !ok {
		return fmt.Errorf("invalid info.length field")
	}

	f.Info.Name, ok = info["name"].(string)
	if !ok {
		return fmt.Errorf("invalid info.name field")
	}

	f.Info.PieceLength, ok = info["piece length"].(int)
	if !ok {
		return fmt.Errorf("invalid info.piece length field")
	}

	f.Info.Pieces, ok = info["pieces"].(string)
	if !ok {
		return fmt.Errorf("invalid info.pieces field")
	}

	return nil
}
