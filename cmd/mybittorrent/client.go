package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

type Client struct {
	PeerId   string
	Port     int
	Torrents map[string]*ClientTorrent
}

type ClientTorrent struct {
	Meta         Meta
	Uploaded     int
	Downloaded   int
	Left         int
	PeerResponse PeerResponse
	Peers        []*Peer
}

type PeerResponse struct {
	Interval int
	Peers    []string
}

type FileResult struct {
	Data  []byte
	Piece Piece
}

type FileWriter struct {
	ch    chan FileResult
	piece Piece
}

func (c *Client) ConnectPeers(filename string) (*ClientTorrent, error) {
	ct, ok := c.Torrents[filename]
	if !ok {
		if err := c.AddTorrentFile(filename); err != nil {
			return nil, err
		}
	}

	if len(ct.PeerResponse.Peers) == 0 {
		if _, err := c.GetPeers(filename); err != nil {
			return nil, fmt.Errorf("failed to get peers: %v+", err)
		}
	}

	for _, peerAddr := range ct.PeerResponse.Peers {
		peer, err := c.Handshake(filename, peerAddr)
		if err != nil {
			continue
		}

		ct.Peers = append(ct.Peers, peer)
	}

	return ct, nil
}

func (c *Client) Close() (err error) {
	for _, ct := range c.Torrents {
		err = errors.Join(err, ct.Close())
	}

	return
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

func (ct *ClientTorrent) getUrl(c Client) (string, error) {
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

	ct.PeerResponse = PeerResponse{
		Interval: resp.Interval,
		Peers:    DecodePeers([]byte(resp.Peers)),
	}

	return ct.PeerResponse, nil
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

func (f *FileResult) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(f.Data)
	return int64(n), err
}

func (f *FileWriter) Write(b []byte) (int, error) {

	f.ch <- FileResult{

		Data: b,

		Piece: f.piece,
	}

	return len(b), nil

}

func (ct *ClientTorrent) Download(out string) error {
	pieces := ct.Meta.Pieces()
	slog.Debug("Starting download", "pieceCnt", len(pieces), "peers", ct.PeerResponse)
	pieceCh := make(chan Piece, len(pieces))
	fileCh := make(chan FileResult)
	wg := sync.WaitGroup{}
	wg.Add(len(pieces))

	for _, piece := range pieces {
		pieceCh <- piece
	}

	for _, peer := range ct.Peers {
		go peer.Download(pieceCh, fileCh)
	}

	go func() {
		wg.Wait()
		close(fileCh)
		close(pieceCh)
	}()

	return ct.WriteFile(pieceCh, fileCh, out, &wg)
}

func (ct *ClientTorrent) Close() (err error) {
	for _, peer := range ct.Peers {
		err = errors.Join(err, peer.Close())
	}

	return
}

func (ct *ClientTorrent) WriteFile(pieceCh chan Piece, fileCh chan FileResult, out string, wg *sync.WaitGroup) error {
	f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer f.Close()

	for fr := range fileCh {
		if _, err = f.Seek(int64(fr.Piece.Index*ct.Meta.Info.PieceLength), io.SeekStart); err != nil {
			slog.Error("failed to seek", "err", err)
			pieceCh <- fr.Piece
			continue
		}

		if _, err = fr.WriteTo(f); err != nil {
			slog.Error("failed to write piece to file", "err", err)
			pieceCh <- fr.Piece
			continue
		}

		if err = f.Sync(); err != nil {
			slog.Error("failed to sync file to disk", "err", err)
			continue
		}

		slog.Debug("writing to file", "piece index", fr.Piece.Index)
		wg.Done()
	}

	return nil
}
