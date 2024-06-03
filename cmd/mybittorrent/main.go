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

		sha := sha1.New()
		if err = bencode.Marshal(sha, meta.Info); err != nil {
			panic(err)
		}

		fmt.Printf("Tracker URL: %s\nLength: %d\nInfo Hash: %x", meta.Announce, meta.Info.Length, sha.Sum(nil))

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
