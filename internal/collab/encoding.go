package collab

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// Yjs WebSocket message types
// Reference: y-protocols/src/protocol.js
const (
	MsgSync      = 0
	MsgAwareness = 1

	// Sync sub-types
	SyncStep1  = 0 // Server -> Client: send state vector, ask for diff
	SyncStep2  = 1 // Client -> Server: send diff (update) based on state vector
	SyncUpdate = 2 // Broadcast: incremental update
)

// VarUint encoding/decoding (unsigned variable-length integers)
// Yjs uses a 7-bit varint encoding: the high bit (0x80) indicates continuation.

// WriteVarUint writes a variable-length unsigned integer to the writer.
func WriteVarUint(w io.Writer, val uint64) error {
	for val > 0x7F {
		b := byte(val&0x7F) | 0x80
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		val >>= 7
	}
	_, err := w.Write([]byte{byte(val & 0x7F)})
	return err
}

// ReadVarUint reads a variable-length unsigned integer from the reader.
func ReadVarUint(r io.Reader) (uint64, error) {
	var val uint64
	var shift uint
	buf := make([]byte, 1)
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		b := buf[0]
		val |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, errors.New("varuint overflow")
		}
	}
	return val, nil
}

// WriteVarUintBytes encodes a varuint into a byte slice.
func WriteVarUintBytes(val uint64) []byte {
	var buf []byte
	for val > 0x7F {
		buf = append(buf, byte(val&0x7F)|0x80)
		val >>= 7
	}
	buf = append(buf, byte(val&0x7F))
	return buf
}

// ReadVarUintFromBytes decodes a varuint from a byte slice, returning the value and bytes consumed.
func ReadVarUintFromBytes(data []byte) (uint64, int, error) {
	var val uint64
	var shift uint
	for i, b := range data {
		val |= uint64(b&0x7F) << shift
		if b&0x80 == 0 {
			return val, i + 1, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, 0, errors.New("varuint overflow")
		}
	}
	return 0, 0, io.ErrUnexpectedEOF
}

// WriteVarBytes writes a length-prefixed byte array (varuint length + data).
func WriteVarBytes(w io.Writer, data []byte) error {
	if err := WriteVarUint(w, uint64(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// ReadVarBytes reads a length-prefixed byte array.
func ReadVarBytes(r io.Reader) ([]byte, error) {
	length, err := ReadVarUint(r)
	if err != nil {
		return nil, err
	}
	if length > math.MaxInt32 {
		return nil, errors.New("varbytes length too large")
	}
	data := make([]byte, length)
	_, err = io.ReadFull(r, data)
	return data, err
}

// EncodeMessage creates a Yjs protocol message with type prefix.
// Format: varuint(messageType) + payload
func EncodeMessage(msgType byte, payload []byte) []byte {
	buf := WriteVarUintBytes(uint64(msgType))
	buf = append(buf, payload...)
	return buf
}

// DecodeMessage reads the message type and returns (type, remaining payload).
func DecodeMessage(data []byte) (byte, []byte, error) {
	if len(data) == 0 {
		return 0, nil, errors.New("empty message")
	}
	msgType, consumed, err := ReadVarUintFromBytes(data)
	if err != nil {
		return 0, nil, err
	}
	return byte(msgType), data[consumed:], nil
}

// EncodeSyncStep1 creates a SyncStep1 message (request state diff based on state vector).
// Payload: varuint(SyncStep1) + varuint(stateVectorLength) + stateVector
func EncodeSyncStep1(stateVector []byte) []byte {
	var payload []byte
	payload = append(payload, WriteVarUintBytes(uint64(SyncStep1))...)
	payload = append(payload, WriteVarUintBytes(uint64(len(stateVector)))...)
	payload = append(payload, stateVector...)
	return EncodeMessage(MsgSync, payload)
}

// EncodeSyncStep2 creates a SyncStep2 message (response with update).
// Payload: varuint(SyncStep2) + varuint(updateLength) + update
func EncodeSyncStep2(update []byte) []byte {
	var payload []byte
	payload = append(payload, WriteVarUintBytes(uint64(SyncStep2))...)
	payload = append(payload, WriteVarUintBytes(uint64(len(update)))...)
	payload = append(payload, update...)
	return EncodeMessage(MsgSync, payload)
}

// EncodeSyncUpdate creates a SyncUpdate message (broadcast incremental update).
// Payload: varuint(SyncUpdate) + varuint(updateLength) + update
func EncodeSyncUpdate(update []byte) []byte {
	var payload []byte
	payload = append(payload, WriteVarUintBytes(uint64(SyncUpdate))...)
	payload = append(payload, WriteVarUintBytes(uint64(len(update)))...)
	payload = append(payload, update...)
	return EncodeMessage(MsgSync, payload)
}

// DecodeSyncMessage parses a sync message payload and returns (subType, data).
func DecodeSyncMessage(payload []byte) (byte, []byte, error) {
	if len(payload) == 0 {
		return 0, nil, errors.New("empty sync payload")
	}
	subType, consumed, err := ReadVarUintFromBytes(payload)
	if err != nil {
		return 0, nil, err
	}
	remaining := payload[consumed:]

	// Read the data length and data
	dataLen, consumed2, err := ReadVarUintFromBytes(remaining)
	if err != nil {
		return 0, nil, err
	}
	remaining = remaining[consumed2:]
	if uint64(len(remaining)) < dataLen {
		return 0, nil, errors.New("sync data truncated")
	}
	return byte(subType), remaining[:dataLen], nil
}

// EncodeAwarenessUpdate creates an awareness message.
// The awareness protocol wraps awareness updates as:
// varuint(MsgAwareness) + varuint(encodedLength) + encoded
func EncodeAwarenessUpdate(encoded []byte) []byte {
	return EncodeMessage(MsgAwareness, append(WriteVarUintBytes(uint64(len(encoded))), encoded...))
}

// EncodeUint32LE encodes a uint32 in little-endian format.
func EncodeUint32LE(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}

// DecodeUint32LE decodes a uint32 from little-endian bytes.
func DecodeUint32LE(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}
