package gotls

import (
	"encoding/binary"
	"io"
	"net"
)

type Packet interface {
	Serialize() []byte
}

type Protocol interface {
	ReadPacket(conn *net.Conn) (Packet, error)
}

type McPacket struct {
	buff []byte
}

func (this *McPacket) Serialize() []byte {
	return this.buff
}

func (this *McPacket) GetLength() uint32 {
	return binary.BigEndian.Uint32(this.buff[0:4])
}

func (this *McPacket) GetBody() []byte {
	return this.buff[4:]
}

func NewMcPacket(buff []byte, hasLengthField bool) *McPacket {
	p := &McPacket{}

	if hasLengthField {
		p.buff = buff

	} else {
		p.buff = make([]byte, 4+len(buff))
		binary.BigEndian.PutUint32(p.buff[0:4], uint32(len(buff)))
		copy(p.buff[4:], buff)
	}

	return p
}

type McProtocol struct {
}

func (this *McProtocol) ReadPacket(conn net.Conn) (Packet, error) {
	var (
		lengthBytes []byte = make([]byte, 4)
		length      uint32
	)

	// read length
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return nil, err
	}
	// if length = binary.BigEndian.Uint32(lengthBytes); length > 4096 {
	// 	return nil, errors.New("the size of packet is larger than the limit")
	// }
	// Removed length limitation.
	length = binary.BigEndian.Uint32(lengthBytes)

	buff := make([]byte, 4+length)
	copy(buff[0:4], lengthBytes)

	// read body ( buff = lengthBytes + body )
	if _, err := io.ReadFull(conn, buff[4:]); err != nil {
		return nil, err
	}

	return NewMcPacket(buff, true), nil
}
