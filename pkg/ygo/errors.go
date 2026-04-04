package ygo

import "fmt"

type YgoError struct {
	Code    ErrorCode
	Message string
}

type ErrorCode int

const (
	ErrIncompleteDocument ErrorCode = iota
	ErrDamagedDocumentJson
	ErrContentSplitNotSupport
	ErrParentNotFound
	ErrRootStructNotFound
	ErrTypeCastError
	ErrUpdateNotFullyConsumed
	ErrInvalidWriteBuffer
)

func (e *YgoError) Error() string {
	return fmt.Sprintf("ygo error [%d]: %s", e.Code, e.Message)
}

func NewIncompleteDocumentError(msg string) error {
	return &YgoError{Code: ErrIncompleteDocument, Message: msg}
}

func NewDamagedDocumentJsonError() error {
	return &YgoError{Code: ErrDamagedDocumentJson, Message: "damaged document json"}
}

func NewContentSplitNotSupportError(offset uint64) error {
	return &YgoError{Code: ErrContentSplitNotSupport, Message: fmt.Sprintf("content split not supported at offset %d", offset)}
}

func NewParentNotFoundError() error {
	return &YgoError{Code: ErrParentNotFound, Message: "parent not found"}
}

func NewUpdateNotFullyConsumedError(remaining int) error {
	return &YgoError{Code: ErrUpdateNotFullyConsumed, Message: fmt.Sprintf("update not fully consumed, %d bytes remaining", remaining)}
}
