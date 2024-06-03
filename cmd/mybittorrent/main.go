package main

import (
	"encoding/json"
	"fmt"
	"os"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

func main() {
	command := os.Args[1]

	switch command {
	case "decode":
		bencodedValue := os.Args[2]

		decoded, err := NewDecoder(bencodedValue).Decode()
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))

	case "info":
		f, err := os.Open(os.Args[2])
		if err != nil {
			fmt.Println("Failed to open file: " + os.Args[2])
		}

		tf := NewFile()

		err = tf.ReadFrom(f)
		if err != nil {
			fmt.Println("Failed to parse file: " + os.Args[2])
			panic(err)
		}

		fmt.Printf("Tracker URL: %s\nLength: %d", tf.Announce, tf.Info.Length)

	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
