package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func createClient(fn string) *Client {
	c := NewClient("00112233445566778899", 6881)

	if err := c.AddTorrentFile(fn); err != nil {
		panic(err)
	}

	return c
}

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]
		decoded, err := bencode.Decode(strings.NewReader(bencodedValue))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

	case "info":
		fn := os.Args[2]
		c := createClient(fn)
		meta := c.Torrents[fn].Meta
		fmt.Printf("Tracker URL: %s\n", meta.Announce)
		fmt.Printf("Length: %d\n", meta.Info.Length)

		infoHash, err := meta.InfoHash()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Info Hash: %x", infoHash)

		fmt.Printf("Piece Length: %d\n", meta.Info.PieceLength)
		fmt.Println("Piece Hashes:")

		for _, h := range meta.PieceHashes() {
			fmt.Printf("%x\n", h)
		}

	case "peers":
		fn := os.Args[2]
		c := createClient(fn)

		pr, err := c.GetPeers(fn)
		if err != nil {
			panic(err)
		}

		for _, peer := range pr.Peers {
			fmt.Println(peer)
		}

	case "handshake":
		fn := os.Args[2]
		c := createClient(fn)
		peerAddr := os.Args[3]

		peer, err := c.Handshake(fn, peerAddr)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Peer ID: %s\n", peer.PeerIdHexString())

	case "download_piece":
		out := os.Args[3]
		fn := os.Args[4]

		index, err := strconv.Atoi(os.Args[5])
		if err != nil {
			panic(err)
		}

		c := createClient(fn)
		pr, err := c.GetPeers(fn)
		if err != nil {
			panic(err)
		}

		var peer *Peer

		for _, peerAddr := range pr.Peers {
			peer, err = c.Handshake(fn, peerAddr)
			if err != nil {
				continue
			}
		}

		if peer == nil {
			panic(fmt.Errorf("no peers found for file: %s", out))
		}

		defer peer.Close()

		err = peer.DownloadPiece(out, index)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Piece %d downloaded to %s.", index, out)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
