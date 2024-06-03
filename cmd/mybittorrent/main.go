package main

import (
	"crypto/sha1"
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
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Println("Failed to open file: ", os.Args[2])
		}

		var meta Meta
		if err = bencode.Unmarshal(f, &meta); err != nil {
			panic(err)
		}

		fmt.Printf("Tracker URL: %s\n", meta.Announce)
		fmt.Printf("Length: %d\n", meta.Info.Length)

		sha := sha1.New()
		if err = bencode.Marshal(sha, meta.Info); err != nil {
			panic(err)
		}

		fmt.Printf("Info Hash: %x", sha.Sum(nil))

		fmt.Printf("Piece Length: %d\n", meta.Info.PieceLength)
		fmt.Println("Piece Hashes:")

		for i := 0; i < len(meta.Info.Pieces); i += 20 {
			fmt.Printf("%x\n", meta.Info.Pieces[i:i+20])
		}

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
