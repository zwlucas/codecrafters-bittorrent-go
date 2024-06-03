package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

type HandshakeMessage []byte

type Peer struct {
	conn      net.Conn
	handshake HandshakeMessage
	ct        *ClientTorrent
	msgCh     chan *IncomingMessage
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

func (p *Peer) DownloadPiece(outFile string, index int) error {
	if index >= p.ct.Meta.PieceCount() {
		return nil
	}

	var data = new(bytes.Buffer)
	blockLens := p.ct.Meta.BlockLens(index)
	var blockIndex = 0

	requestFn := func() error {
		if blockIndex == len(blockLens) {
			return nil
		}

		r := RequestPayload{
			Index:  uint32(index),
			Begin:  uint32(blockIndex * BlockSize),
			Length: blockLens[blockIndex],
		}

		return p.WriteMessage(MessageTypeRequest, r.Bytes())
	}

	for {
		if blockIndex == len(blockLens) {
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

		default:
			return fmt.Errorf("unimplemented message type: %d", msg.MessageType)
		}
	}

	if !p.ct.Meta.CheckHash(index, data.Bytes()) {
		return fmt.Errorf("invalid hash value")
	}

	out, err := os.Create(outFile)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to create file"), err)
	}

	_, err = out.Write(data.Bytes())
	if err != nil {
		return errors.Join(fmt.Errorf("failed to write file"), err)
	}

	err = out.Close()
	if err != nil {
		return errors.Join(fmt.Errorf("failed to close file"), err)
	}

	return nil
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
