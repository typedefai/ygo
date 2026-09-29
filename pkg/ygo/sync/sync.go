// Package sync implements the y-protocols document sync messages:
//
//	0 SyncStep1  — "here is my state vector"
//	1 SyncStep2  — "here is the update you are missing"
//	2 Update     — "here is an incremental update"
//
// Messages are transport-agnostic []byte values; the payload is a V1 Yjs
// update, so they interoperate with y-websocket and y-protocols clients.
package sync

import (
	"bytes"
	"fmt"

	"riguz.com/ygo/internal/lib0"
	"riguz.com/ygo/pkg/ygo"
)

const (
	MsgSyncStep1 = 0
	MsgSyncStep2 = 1
	MsgUpdate    = 2
)

// EncodeSyncStep1 serialises the document's state vector.
func EncodeSyncStep1(doc *ygo.Doc) ([]byte, error) {
	sv, err := ygo.EncodeStateVectorV1(doc.StateVector())
	if err != nil {
		return nil, err
	}
	w := lib0.NewBufferWrite()
	if err := w.WriteVarUint64(uint64(MsgSyncStep1)); err != nil {
		return nil, err
	}
	if err := w.WriteVarUint8Array(sv); err != nil {
		return nil, err
	}
	return w.ToBytes(), nil
}

// EncodeSyncStep2 serialises the update a peer at sv is missing (all structs
// when sv is nil).
func EncodeSyncStep2(doc *ygo.Doc, sv *ygo.StateVector) ([]byte, error) {
	update, err := ygo.EncodeStateAsUpdateV1(doc, sv)
	if err != nil {
		return nil, err
	}
	return EncodeUpdateWithType(MsgSyncStep2, update)
}

// EncodeUpdate wraps a raw V1 update as a MsgUpdate.
func EncodeUpdate(update []byte) ([]byte, error) {
	return EncodeUpdateWithType(MsgUpdate, update)
}

// EncodeUpdateWithType wraps a raw V1 update with an explicit message type.
func EncodeUpdateWithType(msgType uint64, update []byte) ([]byte, error) {
	w := lib0.NewBufferWrite()
	if err := w.WriteVarUint64(msgType); err != nil {
		return nil, err
	}
	if err := w.WriteVarUint8Array(update); err != nil {
		return nil, err
	}
	return w.ToBytes(), nil
}

// ApplySyncMessage decodes and applies a sync message. When the message is a
// SyncStep1 it returns a SyncStep2 reply; otherwise reply is nil.
func ApplySyncMessage(doc *ygo.Doc, msg []byte, origin any) ([]byte, error) {
	r := lib0.NewBufferRead(bytes.NewReader(msg))
	msgType, err := r.ReadVarUint()
	if err != nil {
		return nil, err
	}
	payload, err := r.ReadVarUint8Array()
	if err != nil {
		return nil, err
	}
	return ApplySyncPayload(doc, msgType, payload, origin)
}

// ApplySyncPayload dispatches by message type for transports that already
// parsed the framing.
func ApplySyncPayload(doc *ygo.Doc, msgType uint64, payload []byte, origin any) ([]byte, error) {
	switch msgType {
	case MsgSyncStep1:
		sv, err := ygo.DecodeStateVector(payload)
		if err != nil {
			return nil, err
		}
		return EncodeSyncStep2(doc, &sv)
	case MsgSyncStep2, MsgUpdate:
		return nil, doc.ApplyUpdateV1(payload, origin)
	default:
		return nil, fmt.Errorf("ygo/sync: unknown message type %d", msgType)
	}
}

// ReadSyncMessage returns the message type and raw payload without applying
// anything, for custom dispatchers.
func ReadSyncMessage(msg []byte) (msgType uint64, payload []byte, err error) {
	r := lib0.NewBufferRead(bytes.NewReader(msg))
	msgType, err = r.ReadVarUint()
	if err != nil {
		return 0, nil, err
	}
	payload, err = r.ReadVarUint8Array()
	if err != nil {
		return 0, nil, err
	}
	return msgType, payload, nil
}
