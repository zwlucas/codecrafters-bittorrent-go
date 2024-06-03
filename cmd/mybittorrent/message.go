package main

import (
	"encoding/binary"
	"io"
)

type MessageType byte

type BlockPayload struct {
	Index uint32
	Begin uint32
	Block []byte
}

type IncomingMessage struct {
	Len         uint32
	MessageType MessageType
	Payload     []byte
}

type OutgoingMessage struct {
	MessageType MessageType
	Writer      io.Writer
}

type RequestPayload struct {
	Index  uint32
	Begin  uint32
	Length uint32
}

const (
	MessageTypeChoke MessageType = iota
	MessageTypeUnchoke
	MessageTypeInterested
	MessageTypeNotInterested
	MessageTypeHave
	MessageTypeBitfield
	MessageTypeRequest
	MessageTypePiece
	MessageTypeCancel
)

func (o *OutgoingMessage) Write(b []byte) (int, error) {
	msgLen := 1 + len(b)
	payloadBuff := make([]byte, msgLen+4)
	binary.BigEndian.PutUint32(payloadBuff[0:4], uint32(msgLen))
	payloadBuff[4] = byte(o.MessageType)

	if msgLen > 1 {
		copy(payloadBuff[5:], b)
	}

	return o.Writer.Write(payloadBuff)
}

func (r RequestPayload) Bytes() []byte {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[0:4], r.Index)
	binary.BigEndian.PutUint32(buf[4:8], r.Begin)
	binary.BigEndian.PutUint32(buf[8:12], r.Length)
	return buf
}

func (p *BlockPayload) Write(b []byte) (int, error) {
	p.Index = binary.BigEndian.Uint32(b[0:4])
	p.Begin = binary.BigEndian.Uint32(b[4:8])
	p.Block = b[8:]
	return len(b), nil
}

func (p *BlockPayload) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(p.Block)
	return int64(n), err
}
