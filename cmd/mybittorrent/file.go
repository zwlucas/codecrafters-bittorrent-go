package main

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
