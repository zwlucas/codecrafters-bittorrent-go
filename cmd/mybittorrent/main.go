package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

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
		c := NewClient("00112233445566778899", 6881)
		tf, err := c.AddTorrentFile(os.Args[2])
		if err != nil {
			panic(err)
		}

		fmt.Printf("Tracker URL: %s\n", tf.Meta.Announce)
		fmt.Printf("Length: %d\n", tf.Meta.Info.Length)

		infoHash, err := tf.Meta.InfoHash()
		if err != nil {
			panic(err)
		}

		fmt.Printf("Info Hash: %x", infoHash)

		fmt.Printf("Piece Length: %d\n", tf.Meta.Info.PieceLength)
		fmt.Println("Piece Hashes:")

		for _, h := range tf.Meta.PieceHashes() {
			fmt.Printf("%x\n", h)
		}

	case "peers":
		c := NewClient("00112233445566778899", 6881)

		_, err := c.AddTorrentFile(os.Args[2])
		if err != nil {
			panic(err)
		}

		pr, err := c.GetPeers(os.Args[2])

		if err != nil {
			panic(err)
		}

		for _, peer := range pr.Peers {
			fmt.Println(peer)
		}

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
