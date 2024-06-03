package main

import (
	"bytes"
	"crypto/sha1"
	"math"

	bencode "github.com/jackpal/bencode-go"
)

const BlockSize = 16 * 1024

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

func (m Meta) CheckHash(pieceIndex int, data []byte) bool {
	sha := sha1.New()

	if _, err := bytes.NewBuffer(data).WriteTo(sha); err != nil {
		return false
	}

	return bytes.Equal([]byte(m.Info.Pieces[pieceIndex*20:pieceIndex*20+20]), sha.Sum(nil))
}

func (m Meta) PieceCount() int {
	return len(m.Info.Pieces) / 20
}

func (m Meta) PieceLens() []int {
	pieceCnt := m.PieceCount()
	pieces := make([]int, pieceCnt)

	for i := 0; i < pieceCnt; i++ {
		if i < pieceCnt-1 {
			pieces[i] = m.Info.PieceLength
		} else {
			pieces[i] = m.Info.Length - i*m.Info.PieceLength
		}
	}

	return pieces
}

func (m Meta) BlockLens(pieceIdx int) []uint32 {
	pieceLen := m.PieceLens()[pieceIdx]
	blockCnt := int(math.Ceil(float64(pieceLen) / float64(BlockSize)))
	blocks := make([]uint32, blockCnt)

	for i := 0; i < blockCnt; i++ {
		if i < blockCnt-1 {
			blocks[i] = uint32(BlockSize)
		} else {
			blocks[i] = uint32(pieceLen - i*BlockSize)
		}
	}

	return blocks
}
