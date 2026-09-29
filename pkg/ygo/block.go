package ygo

import (
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

type ClientID uint64

type Clock = uint64

type ID struct {
	Client ClientID
	Clock  Clock
}

type Block interface {
	LastId() ID
	AsItem() (*Item, error)
	IsDeleted() bool
	Id() ID
	Len() uint64
	SameType(other *Block) bool
	IsGc() bool
	IsItem() bool
	Contains(id ID) bool
}

type Item struct {
	ID          ID
	Length      uint64
	Left        *Item
	Right       *Item
	Origin      *ID
	RightOrigin *ID
	Content     ItemContent
	Parent      *Parent
	ParentSub   *string
	Info        ItemFlags

	// parentType is the live containing type, resolved during integration.
	parentType *abstractType
}

func (i *Item) Contains(id ID) bool {
	length := i.Length
	if length == 0 {
		length = i.Len()
	}
	return i.ID.Client == id.Client &&
		id.Clock >= i.ID.Clock &&
		id.Clock < i.ID.Clock+length
}

func (i *Item) IsDeleted() bool {
	return i.Info.IsDeleted()
}

func (i *Item) IsCountable() bool {
	return i.Info.IsCountable()
}

func (i *Item) MarkAsDeleted() {
	i.Info.SetDeleted()
}

func (i *Item) Len() uint64 {
	return i.Content.ClockLen()
}

func (i *Item) LastId() ID {
	length := i.Length
	if length == 0 {
		length = i.Len()
	}
	return ID{
		Client: i.ID.Client,
		Clock:  i.ID.Clock + length - 1,
	}
}

const (
	HAS_ORIGIN       uint8 = 0b1000_0000
	HAS_RIGHT_ORIGIN uint8 = 0b0100_0000
	HAS_PARENT_SUB   uint8 = 0b0010_0000
	HAS_SIBLING      uint8 = 0b1100_0000 // HAS_ORIGIN | HAS_RIGHT_ORIGIN
)

func (i *Item) ItemInfo() uint8 {
	return i.ItemInfoFor(i.Origin, i.RightOrigin)
}

// ItemInfoFor computes the info byte for the given effective origins (offset
// encoding re-bases the origin).
func (i *Item) ItemInfoFor(origin, rightOrigin *ID) uint8 {
	var info uint8 = 0
	if origin != nil {
		info |= HAS_ORIGIN
	}

	if rightOrigin != nil {
		info |= HAS_RIGHT_ORIGIN
	}

	if i.ParentSub != nil {
		info |= HAS_PARENT_SUB
	}
	info |= i.Content.GetRefNumber() & 0b1111
	return info
}

// ReadItem reads an Item from the decoder. id is the item's ID, info is the
// info byte already read, and first5Bit is info & 0x1f (the content tag).
func ReadItem(decoder Decoder, id ID, info uint8, first5Bit uint8) (*Item, error) {
	hasLeftID := info&HAS_ORIGIN != 0
	hasRightID := info&HAS_RIGHT_ORIGIN != 0
	hasParentSub := info&HAS_PARENT_SUB != 0
	hasNotSibling := info&HAS_SIBLING == 0

	var origin *ID
	if hasLeftID {
		leftID, err := decoder.ReadLeftId()
		if err != nil {
			return nil, err
		}
		origin = &leftID
	}

	var rightOrigin *ID
	if hasRightID {
		rightID, err := decoder.ReadRightId()
		if err != nil {
			return nil, err
		}
		rightOrigin = &rightID
	}

	var parent *Parent
	if hasNotSibling {
		hasParent, err := decoder.ReadParentInfo()
		if err != nil {
			return nil, err
		}
		if hasParent {
			name, err := decoder.ReadVarString()
			if err != nil {
				return nil, err
			}
			p := ParentFromString(name)
			parent = &p
		} else {
			pid, err := decoder.ReadLeftId()
			if err != nil {
				return nil, err
			}
			p := ParentFromID(pid)
			parent = &p
		}
	}

	var parentSub *string
	if hasNotSibling && hasParentSub {
		s, err := decoder.ReadVarString()
		if err != nil {
			return nil, err
		}
		parentSub = &s
	}

	content, err := ReadContent(decoder, first5Bit)
	if err != nil {
		return nil, err
	}

	item := &Item{
		ID:          id,
		Origin:      origin,
		RightOrigin: rightOrigin,
		Parent:      parent,
		ParentSub:   parentSub,
		Content:     content,
		Info:        NewItemFlags(0),
	}

	if item.Content.IsCountable() {
		item.Info.SetCountable()
	}
	if _, ok := item.Content.(*DeletedContent); ok {
		item.Info.SetDeleted()
	}

	return item, nil
}

// WriteItem writes the item to the encoder. offset > 0 encodes only the
// suffix starting at that clock offset (Yjs Item.write(encoder, offset)); the
// origin is then re-based to this item's last known id.
func (i *Item) WriteItem(encoder Encoder, offset uint64) error {
	origin := i.Origin
	rightOrigin := i.RightOrigin
	if offset > 0 {
		oc := i.ID.Clock + offset - 1
		origin = &ID{Client: i.ID.Client, Clock: oc}
	}
	info := i.ItemInfoFor(origin, rightOrigin)
	hasNotSibling := info&HAS_SIBLING == 0

	if err := encoder.WriteInfo(info); err != nil {
		return err
	}

	if origin != nil {
		if err := encoder.WriteLeftId(*origin); err != nil {
			return err
		}
	}
	if rightOrigin != nil {
		if err := encoder.WriteRightId(*rightOrigin); err != nil {
			return err
		}
	}

	if hasNotSibling {
		if i.Parent == nil {
			return NewParentNotFoundError()
		}
		if i.Parent.Named != nil {
			if err := encoder.WriteParentInfo(true); err != nil {
				return err
			}
			if err := encoder.WriteVarString(i.Parent.Named); err != nil {
				return err
			}
		} else if i.Parent.ID != nil {
			if err := encoder.WriteParentInfo(false); err != nil {
				return err
			}
			if err := encoder.WriteLeftId(*i.Parent.ID); err != nil {
				return err
			}
		}

		if i.ParentSub != nil {
			if err := encoder.WriteVarString(i.ParentSub); err != nil {
				return err
			}
		}
	}

	return i.Content.Write(encoder, offset)
}

// SplitAt splits the item at offset in place: the receiver becomes the left
// half (keeping its identity so parent.start/map pointers stay valid) and the
// new right half is returned, mirroring Yjs splitItem.
func (i *Item) SplitAt(offset uint64) (*Item, error) {
	leftContent, rightContent, err := i.Content.Split(offset)
	if err != nil {
		return nil, err
	}

	rightItem := &Item{
		ID:          ID{Client: i.ID.Client, Clock: i.ID.Clock + offset},
		Origin:      &ID{Client: i.ID.Client, Clock: i.ID.Clock + offset - 1},
		RightOrigin: i.RightOrigin,
		Parent:      i.Parent,
		ParentSub:   i.ParentSub,
		Content:     rightContent,
		Info:        i.Info,
		parentType:  i.parentType,
	}
	// The halves inherit the original's neighbours; repoint the right neighbour.
	rightItem.Right = i.Right
	if i.Right != nil {
		i.Right.Left = rightItem
	}
	rightItem.Left = i

	i.Content = leftContent
	i.Right = rightItem

	return rightItem, nil
}

type GC struct {
}

type BlockRange struct {
	ID  ID
	Len uint64
}

type ItemContent interface {
	GetRefNumber() uint8
	IsCountable() bool
	ClockLen() uint64
	Read(decoder Decoder) error
	Write(encoder Encoder, offset uint64) error
	Split(offset uint64) (ItemContent, ItemContent, error)
	Splice(offset uint64) ItemContent
	Integrate(txn *Transaction, item *Item)
	Delete(txn *Transaction)
	MergeWith(right ItemContent) bool
}

// YTypeKind represents the kind of Y type
type YTypeKind uint64

const (
	YTypeArray       YTypeKind = 0
	YTypeMap         YTypeKind = 1
	YTypeText        YTypeKind = 2
	YTypeXMLElement  YTypeKind = 3
	YTypeXMLFragment YTypeKind = 4
	YTypeXMLHook     YTypeKind = 5
	YTypeXMLText     YTypeKind = 6
	YTypeUnknown     YTypeKind = 255
)

func YTypeKindFromU64(v uint64) YTypeKind {
	switch v {
	case 0:
		return YTypeArray
	case 1:
		return YTypeMap
	case 2:
		return YTypeText
	case 3:
		return YTypeXMLElement
	case 4:
		return YTypeXMLFragment
	case 5:
		return YTypeXMLHook
	case 6:
		return YTypeXMLText
	default:
		return YTypeUnknown
	}
}

// YTypeRef represents a reference to a Y type (used in TypeContent)
type YTypeRef struct {
	Kind    YTypeKind
	TagName *string // only for XMLElement and XMLHook
}

func NewYTypeRef(kind YTypeKind, tagName *string) YTypeRef {
	return YTypeRef{Kind: kind, TagName: tagName}
}

const (
	BLOCK_GC_REF_NUMBER           uint8 = 0
	BLOCK_ITEM_DELETED_REF_NUMBER uint8 = 1
	BLOCK_ITEM_JSON_REF_NUMBER    uint8 = 2
	BLOCK_ITEM_BINARY_REF_NUMBER  uint8 = 3
	BLOCK_ITEM_STRING_REF_NUMBER  uint8 = 4
	BLOCK_ITEM_EMBED_REF_NUMBER   uint8 = 5
	BLOCK_ITEM_FORMAT_REF_NUMBER  uint8 = 6
	BLOCK_ITEM_TYPE_REF_NUMBER    uint8 = 7
	BLOCK_ITEM_ANY_REF_NUMBER     uint8 = 8
	BLOCK_ITEM_DOC_REF_NUMBER     uint8 = 9
	BLOCK_SKIP_REF_NUMBER         uint8 = 10
	BLOCK_ITEM_MOVE_REF_NUMBER    uint8 = 11
)

// --- DeletedContent ---

type DeletedContent struct {
	Len uint64
}

func (c *DeletedContent) GetRefNumber() uint8 { return BLOCK_ITEM_DELETED_REF_NUMBER }
func (c *DeletedContent) IsCountable() bool   { return false }
func (c *DeletedContent) ClockLen() uint64    { return c.Len }

func (c *DeletedContent) Read(decoder Decoder) error {
	length, err := decoder.ReadLen()
	if err != nil {
		return err
	}
	c.Len = length
	return nil
}

func (c *DeletedContent) Write(encoder Encoder, offset uint64) error {
	return encoder.WriteLen(c.Len - offset)
}

func (c *DeletedContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return &DeletedContent{Len: offset}, &DeletedContent{Len: c.Len - offset}, nil
}

// --- JsonContent ---

// JsonContent mirrors Yjs ContentJSON (wire tag 2): a countable run of
// arbitrary lib0 Any values, encoded as VarUint(count) followed by count
// Any payloads. Each element is one clock unit.
type JsonContent struct {
	Data []Any
}

func (c *JsonContent) GetRefNumber() uint8 { return BLOCK_ITEM_JSON_REF_NUMBER }
func (c *JsonContent) IsCountable() bool   { return true }
func (c *JsonContent) ClockLen() uint64    { return uint64(len(c.Data)) }

func (c *JsonContent) Read(decoder Decoder) error {
	data, err := ReadMultipleAny(decoder)
	if err != nil {
		return err
	}
	c.Data = data
	return nil
}

func (c *JsonContent) Write(encoder Encoder, offset uint64) error {
	if err := encoder.WriteLen(uint64(len(c.Data)) - offset); err != nil {
		return err
	}
	for _, a := range c.Data[offset:] {
		if err := encoder.WriteAnyValue(a); err != nil {
			return err
		}
	}
	return nil
}

func (c *JsonContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	left := make([]Any, offset)
	copy(left, c.Data[:offset])
	right := make([]Any, uint64(len(c.Data))-offset)
	copy(right, c.Data[offset:])
	return &JsonContent{Data: left}, &JsonContent{Data: right}, nil
}

// --- BinaryContent ---

type BinaryContent struct {
	Data []byte
}

func (c *BinaryContent) GetRefNumber() uint8 { return BLOCK_ITEM_BINARY_REF_NUMBER }
func (c *BinaryContent) IsCountable() bool   { return true }
func (c *BinaryContent) ClockLen() uint64    { return 1 }

func (c *BinaryContent) Read(decoder Decoder) error {
	data, err := decoder.ReadVarUint8Array()
	if err != nil {
		return err
	}
	c.Data = data
	return nil
}

func (c *BinaryContent) Write(encoder Encoder, _ uint64) error {
	return encoder.WriteVarUint8Array(c.Data)
}

func (c *BinaryContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// --- StringContent ---

type StringContent struct {
	Data string
}

func (c *StringContent) GetRefNumber() uint8 { return BLOCK_ITEM_STRING_REF_NUMBER }
func (c *StringContent) IsCountable() bool   { return true }
func (c *StringContent) ClockLen() uint64 {
	// length in UTF-16 code units
	u16 := utf16.Encode([]rune(c.Data))
	return uint64(len(u16))
}

func (c *StringContent) Read(decoder Decoder) error {
	s, err := decoder.ReadVarString()
	if err != nil {
		return err
	}
	c.Data = s
	return nil
}

func (c *StringContent) Write(encoder Encoder, offset uint64) error {
	s := c.Data
	if offset > 0 {
		_, s = splitAsUtf16Str(c.Data, offset)
	}
	return encoder.WriteVarString(&s)
}

func (c *StringContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	left, right := splitAsUtf16Str(c.Data, offset)
	return &StringContent{Data: left}, &StringContent{Data: right}, nil
}

// splitAsUtf16Str splits a string at a UTF-16 offset
func splitAsUtf16Str(s string, offset uint64) (string, string) {
	var utf16Offset uint64
	var utf8Offset int
	for _, ch := range s {
		utf16Offset += uint64(utf16Len(ch))
		utf8Offset += utf8.RuneLen(ch)
		if utf16Offset >= offset {
			break
		}
	}
	return s[:utf8Offset], s[utf8Offset:]
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// --- EmbedContent ---

type EmbedContent struct {
	Data Any
}

func (c *EmbedContent) GetRefNumber() uint8 { return BLOCK_ITEM_EMBED_REF_NUMBER }
func (c *EmbedContent) IsCountable() bool   { return true }
func (c *EmbedContent) ClockLen() uint64    { return 1 }

func (c *EmbedContent) Read(decoder Decoder) error {
	// V1: legacy JSON text; V2: lib0 Any. Both route through ReadJson.
	v, err := decoder.ReadJson()
	if err != nil {
		return err
	}
	c.Data = v
	return nil
}

func (c *EmbedContent) Write(encoder Encoder, _ uint64) error {
	return encoder.WriteJson(c.Data)
}

func (c *EmbedContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// --- FormatContent ---

type FormatContent struct {
	Key   string
	Value Any
}

func (c *FormatContent) GetRefNumber() uint8 { return BLOCK_ITEM_FORMAT_REF_NUMBER }
func (c *FormatContent) IsCountable() bool   { return false }
func (c *FormatContent) ClockLen() uint64    { return 1 }

func (c *FormatContent) Read(decoder Decoder) error {
	key, err := decoder.ReadKey()
	if err != nil {
		return err
	}
	c.Key = *key
	val, err := decoder.ReadJson()
	if err != nil {
		return err
	}
	c.Value = val
	return nil
}

func (c *FormatContent) Write(encoder Encoder, _ uint64) error {
	if err := encoder.WriteKey(&c.Key); err != nil {
		return err
	}
	return encoder.WriteJson(c.Value)
}

func (c *FormatContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// --- TypeContent ---

type TypeContent struct {
	TypeRef YTypeRef
	// typ is the live nested type, bound on integration.
	typ *abstractType
}

func (c *TypeContent) GetRefNumber() uint8 { return BLOCK_ITEM_TYPE_REF_NUMBER }
func (c *TypeContent) IsCountable() bool   { return true }
func (c *TypeContent) ClockLen() uint64    { return 1 }

func (c *TypeContent) Read(decoder Decoder) error {
	typeRef, err := decoder.ReadTypeRef()
	if err != nil {
		return err
	}
	kind := YTypeKindFromU64(uint64(typeRef))
	if kind == YTypeUnknown {
		return NewIncompleteDocumentError(fmt.Sprintf("unknown y type: %d", typeRef))
	}
	var tagName *string
	if kind == YTypeXMLElement || kind == YTypeXMLHook {
		// YXmlElement._write/readYXmlElement use writeKey/readKey, not the
		// generic string path.
		tagName, err = decoder.ReadKey()
		if err != nil {
			return err
		}
	}
	c.TypeRef = NewYTypeRef(kind, tagName)
	return nil
}

func (c *TypeContent) Write(encoder Encoder, _ uint64) error {
	if err := encoder.WriteTypeRef(uint8(c.TypeRef.Kind)); err != nil {
		return err
	}
	if c.TypeRef.Kind == YTypeXMLElement || c.TypeRef.Kind == YTypeXMLHook {
		if c.TypeRef.TagName != nil {
			if err := encoder.WriteKey(c.TypeRef.TagName); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *TypeContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// --- AnyContent ---

type AnyContent struct {
	Data []Any
}

func (c *AnyContent) GetRefNumber() uint8 { return BLOCK_ITEM_ANY_REF_NUMBER }
func (c *AnyContent) IsCountable() bool   { return true }
func (c *AnyContent) ClockLen() uint64    { return uint64(len(c.Data)) }

func (c *AnyContent) Read(decoder Decoder) error {
	data, err := ReadMultipleAny(decoder)
	if err != nil {
		return err
	}
	c.Data = data
	return nil
}

func (c *AnyContent) Write(encoder Encoder, offset uint64) error {
	if err := encoder.WriteLen(uint64(len(c.Data)) - offset); err != nil {
		return err
	}
	for _, a := range c.Data[offset:] {
		if err := encoder.WriteAnyValue(a); err != nil {
			return err
		}
	}
	return nil
}

func (c *AnyContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	left := make([]Any, offset)
	copy(left, c.Data[:offset])
	right := make([]Any, uint64(len(c.Data))-offset)
	copy(right, c.Data[offset:])
	return &AnyContent{Data: left}, &AnyContent{Data: right}, nil
}

// --- DocContent ---

type DocContent struct {
	Guid string
	Opts Any
}

func (c *DocContent) GetRefNumber() uint8 { return BLOCK_ITEM_DOC_REF_NUMBER }
func (c *DocContent) IsCountable() bool   { return true }
func (c *DocContent) ClockLen() uint64    { return 1 }

func (c *DocContent) Read(decoder Decoder) error {
	guid, err := decoder.ReadVarString()
	if err != nil {
		return err
	}
	c.Guid = guid
	opts, err := ReadAny(decoder)
	if err != nil {
		return err
	}
	c.Opts = opts
	return nil
}

func (c *DocContent) Write(encoder Encoder, _ uint64) error {
	if err := encoder.WriteVarString(&c.Guid); err != nil {
		return err
	}
	return WriteAny(encoder, c.Opts)
}

func (c *DocContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// --- MoveContent ---

type MoveContent struct{}

func (c *MoveContent) GetRefNumber() uint8 { return BLOCK_ITEM_MOVE_REF_NUMBER }
func (c *MoveContent) IsCountable() bool   { return false }
func (c *MoveContent) ClockLen() uint64    { return 1 }
func (c *MoveContent) Read(decoder Decoder) error {
	return NewIncompleteDocumentError("move content read not supported")
}
func (c *MoveContent) Write(encoder Encoder, _ uint64) error {
	return NewIncompleteDocumentError("move content write not supported")
}
func (c *MoveContent) Split(offset uint64) (ItemContent, ItemContent, error) {
	return nil, nil, NewContentSplitNotSupportError(offset)
}

// ReadContent reads an ItemContent from the decoder based on the tag type
func ReadContent(decoder Decoder, tagType uint8) (ItemContent, error) {
	var content ItemContent
	switch tagType {
	case BLOCK_ITEM_DELETED_REF_NUMBER:
		content = &DeletedContent{}
	case BLOCK_ITEM_JSON_REF_NUMBER:
		content = &JsonContent{}
	case BLOCK_ITEM_BINARY_REF_NUMBER:
		content = &BinaryContent{}
	case BLOCK_ITEM_STRING_REF_NUMBER:
		content = &StringContent{}
	case BLOCK_ITEM_EMBED_REF_NUMBER:
		content = &EmbedContent{}
	case BLOCK_ITEM_FORMAT_REF_NUMBER:
		content = &FormatContent{}
	case BLOCK_ITEM_TYPE_REF_NUMBER:
		content = &TypeContent{}
	case BLOCK_ITEM_ANY_REF_NUMBER:
		content = &AnyContent{}
	case BLOCK_ITEM_DOC_REF_NUMBER:
		content = &DocContent{}
	default:
		return nil, NewIncompleteDocumentError(fmt.Sprintf("unknown content type: %d", tagType))
	}
	if err := content.Read(decoder); err != nil {
		return nil, err
	}
	return content, nil
}

const (
	ITEM_FLAG_MARKED    uint8 = 0b0000_1000
	ITEM_FLAG_DELETED   uint8 = 0b0000_0100
	ITEM_FLAG_COUNTABLE uint8 = 0b0000_0010
	ITEM_FLAG_KEEP      uint8 = 0b0000_0001
)

type ItemFlags struct {
	flags uint8
}

func NewItemFlags(source uint8) ItemFlags {
	return ItemFlags{flags: source}
}

func (i *ItemFlags) Into() uint8 {
	return i.flags
}

func (i *ItemFlags) Set(value uint8) {
	i.flags |= value
}

func (i *ItemFlags) Clear(value uint8) {
	i.flags &= ^value
}

func (i *ItemFlags) Check(value uint8) bool {
	return i.flags&value == value
}

func (i *ItemFlags) IsKeep() bool {
	return i.Check(ITEM_FLAG_KEEP)
}

func (i *ItemFlags) IsCountable() bool {
	return i.Check(ITEM_FLAG_COUNTABLE)
}

func (i *ItemFlags) IsDeleted() bool {
	return i.Check(ITEM_FLAG_DELETED)
}

func (i *ItemFlags) IsMarked() bool {
	return i.Check(ITEM_FLAG_MARKED)
}

func (i *ItemFlags) SetCountable() {
	i.Set(ITEM_FLAG_COUNTABLE)
}

func (i *ItemFlags) ClearCountable() {
	i.Clear(ITEM_FLAG_COUNTABLE)
}

func (i *ItemFlags) SetDeleted() {
	i.Set(ITEM_FLAG_DELETED)
}
