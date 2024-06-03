package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	bencode "github.com/jackpal/bencode-go"
)

type Client struct {
	PeerId   string
	Port     int
	Torrents map[string]*ClientTorrent
}

type ClientTorrent struct {
	Meta       Meta
	Uploaded   int
	Downloaded int
	Left       int
}

type PeerResponse struct {
	Interval int
	Peers    []string
}

func NewClient(peerId string, port int) *Client {
	return &Client{
		PeerId:   peerId,
		Port:     port,
		Torrents: make(map[string]*ClientTorrent),
	}
}

func (c *Client) AddTorrentFile(filename string) error {
	f, err := os.Open(filename)
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			slog.Error("Failed to close file", "filename", filename)
		}
	}(f)

	if err != nil {
		fmt.Println("Failed to open file: ", os.Args[2])
	}

	var meta Meta

	if err = bencode.Unmarshal(f, &meta); err != nil {
		return err
	}

	c.Torrents[filename] = &ClientTorrent{
		Meta:       meta,
		Uploaded:   0,
		Downloaded: 0,
		Left:       meta.Info.Length,
	}

	return nil
}

func (ct ClientTorrent) getUrl(c Client) (string, error) {
	u, err := url.Parse(ct.Meta.Announce)
	if err != nil {
		return "", err
	}

	q := u.Query()

	infoHash, err := ct.Meta.InfoHash()
	if err != nil {
		return "", err
	}

	q.Set("info_hash", string(infoHash))
	q.Set("peer_id", c.PeerId)
	q.Set("port", strconv.Itoa(c.Port))
	q.Set("uploaded", strconv.Itoa(ct.Uploaded))
	q.Set("downloaded", strconv.Itoa(ct.Downloaded))
	q.Set("left", strconv.Itoa(ct.Left))
	q.Set("compact", "1")
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func DecodePeers(b []byte) []string {
	val := make([]string, 0, len(b)/6)
	for i := 0; i < len(b); i += 6 {
		val = append(val, fmt.Sprintf("%d.%d.%d.%d:%d", b[i], b[i+1], b[i+2], b[i+3], binary.BigEndian.Uint16(b[i+4:i+6])))
	}

	return val
}

func (c *Client) GetPeers(filename string) (PeerResponse, error) {
	ct, ok := c.Torrents[filename]
	if !ok {
		return PeerResponse{}, fmt.Errorf("missing file from client: %s", filename)
	}

	u, err := ct.getUrl(*c)
	if err != nil {
		return PeerResponse{}, err
	}

	getResp, err := http.Get(u)
	if err != nil {
		return PeerResponse{}, err
	}

	var resp struct {
		Interval int
		Peers    string
	}

	err = bencode.Unmarshal(getResp.Body, &resp)
	if err != nil {
		return PeerResponse{}, err
	}

	return PeerResponse{
		Interval: resp.Interval,
		Peers:    DecodePeers([]byte(resp.Peers)),
	}, err
}

func (c *Client) Handshake(filename, peerAddr string) (*Peer, error) {
	ct, ok := c.Torrents[filename]
	if !ok {
		return nil, fmt.Errorf("missing torrent file: %s", filename)
	}

	buf := new(bytes.Buffer)
	buf.WriteByte(19)
	buf.WriteString("BitTorrent protocol")
	buf.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0})

	hash, err := ct.Meta.InfoHash()
	if err != nil {
		return nil, err
	}

	buf.Write(hash)
	buf.WriteString(c.PeerId)

	conn, err := net.Dial("tcp", peerAddr)

	if err != nil {
		return nil, err
	}

	_, err = buf.WriteTo(conn)
	if err != nil {
		return nil, err
	}

	respBuf := make([]byte, 68)

	_, err = io.ReadFull(conn, respBuf)

	peer := &Peer{
		conn:      conn,
		handshake: respBuf,
		ct:        ct,
	}

	if !bytes.Equal(peer.InfoHash(), hash) {
		err := conn.Close()
		if err != nil {
			slog.Error("Failed to close peer connection", "remoteAddr", peer.conn.RemoteAddr())
		}

		return nil, fmt.Errorf("invalid info hash from peer: %x, addr: %s", peer.InfoHash(), peerAddr)
	}

	return peer, nil
}
