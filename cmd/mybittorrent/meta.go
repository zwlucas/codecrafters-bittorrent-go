package main

import (
	"crypto/sha1"

	bencode "github.com/jackpal/bencode-go"
)

type Meta struct {
	Announce string   `bencode:"announce"`
	Info     FileInfo `bencode:"info"`
}

type FileInfo struct {
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
	PieceLength int    `bencode:"piece length"`
	Pieces      string `bencode:"pieces"`
}

func (m Meta) InfoHash() ([]byte, error) {
	sha := sha1.New()
	if err := bencode.Marshal(sha, m.Info); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

func (m Meta) PieceHashes() []string {
	hashes := make([]string, 0, len(m.Info.Pieces)/20)
	for i := 0; i < len(m.Info.Pieces); i += 20 {
		hashes = append(hashes, m.Info.Pieces[i:i+20])
	}

	return hashes
}
