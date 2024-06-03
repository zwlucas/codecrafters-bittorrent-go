package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
)

type HandshakeMessage []byte

type Peer struct {
	conn      net.Conn
	handshake HandshakeMessage
	ct        *ClientTorrent
	unchoked  bool
}

func (p *Peer) PeerIdHexString() string {
	return hex.EncodeToString(p.handshake[48:])
}

func (p *Peer) InfoHash() []byte {
	return p.handshake[28:48]
}

func (p *Peer) Close() error {
	err := p.conn.Close()
	if err != nil {
		return errors.Join(fmt.Errorf("failed to close peer connection: %s", p.conn.RemoteAddr()), err)
	}

	return nil
}

func (p *Peer) DownloadPiece(w io.Writer, piece Piece) error {
	if piece.Index >= p.ct.Meta.PieceCount() {
		return nil
	}

	var data = new(bytes.Buffer)
	var blockIndex = 0

	requestFn := func() error {
		if blockIndex == len(piece.Blocks) {
			return nil
		}

		r := RequestPayload{
			Index:  uint32(piece.Index),
			Begin:  uint32(blockIndex * BlockSize),
			Length: piece.Blocks[blockIndex],
		}

		return p.WriteMessage(MessageTypeRequest, r.Bytes())
	}

	if p.unchoked {
		if err := requestFn(); err != nil {
			return err
		}
	}

	for {
		if blockIndex == len(piece.Blocks) {
			break
		}

		msg, err := p.ReadMessage()
		if err != nil {
			return err
		}

		switch msg.MessageType {
		case MessageTypeBitfield:
			if err != nil {
				return err
			}

			err = p.WriteMessage(MessageTypeInterested, nil)
			if err != nil {
				return err
			}

		case MessageTypeUnchoke:
			if err = requestFn(); err != nil {
				return err
			}
			p.unchoked = true

		case MessageTypePiece:
			var block BlockPayload

			_, err = block.Write(msg.Payload)
			if err != nil {
				return err
			}

			_, err = data.Write(block.Block)
			if err != nil {
				return err
			}

			blockIndex++

			if err = requestFn(); err != nil {
				return err
			}

		case MessageTypeChoke:
			p.unchoked = false

		default:
			return fmt.Errorf("unimplemented message type: %d", msg.MessageType)
		}
	}

	slog.Debug("piece downloaded", "piece index", piece.Index)

	if err := piece.CheckHash(data.Bytes()); err != nil {
		return fmt.Errorf("invalid hash value: %v", err)
	}

	slog.Debug("hash ok", "piece index", piece.Index)
	_, err := w.Write(data.Bytes())
	slog.Debug("written to writer", "piece index", piece.Index)
	return err
}

func (p *Peer) ReadMessage() (*IncomingMessage, error) {
	lenBuf := make([]byte, 4)

	_, err := p.conn.Read(lenBuf)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to read message length"), err)
	}

	msgLen := binary.BigEndian.Uint32(lenBuf) - 1
	typeBuf := make([]byte, 1)

	_, err = p.conn.Read(typeBuf)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to read message type"), err)
	}

	msgType := MessageType(typeBuf[0])
	if msgLen == 0 {
		return &IncomingMessage{
			Len:         msgLen,
			MessageType: msgType,
		}, nil
	}

	payloadBuf := make([]byte, msgLen)

	_, err = io.ReadFull(p.conn, payloadBuf)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to read message payload"), err)
	}

	return &IncomingMessage{
		Len:         msgLen,
		MessageType: msgType,
		Payload:     payloadBuf,
	}, nil
}

func (p *Peer) WriteMessage(t MessageType, payload []byte) error {
	msg := &OutgoingMessage{
		MessageType: t,
		Writer:      p.conn,
	}

	_, err := msg.Write(payload)
	return err
}

func (p *Peer) Download(pieceCh chan Piece, fileCh chan FileResult) {
	for piece := range pieceCh {
		fw := FileWriter{
			ch:    fileCh,
			piece: piece,
		}

		err := p.DownloadPiece(&fw, piece)
		if err != nil {
			slog.Error("failed to download piece", "piece", piece, "err", err)
			pieceCh <- piece
			continue
		}

		slog.Debug("downloaded piece", "piece index", piece.Index, "left", len(pieceCh))
	}
}
