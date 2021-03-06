package abi

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *KYCAddress) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "t":
			z.TradeAddress, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "TradeAddress")
				return
			}
		case "c":
			z.Comment, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z KYCAddress) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "t"
	err = en.Append(0x83, 0xa1, 0x74)
	if err != nil {
		return
	}
	err = en.WriteString(z.TradeAddress)
	if err != nil {
		err = msgp.WrapError(err, "TradeAddress")
		return
	}
	// write "c"
	err = en.Append(0xa1, 0x63)
	if err != nil {
		return
	}
	err = en.WriteString(z.Comment)
	if err != nil {
		err = msgp.WrapError(err, "Comment")
		return
	}
	// write "v"
	err = en.Append(0xa1, 0x76)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Valid)
	if err != nil {
		err = msgp.WrapError(err, "Valid")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z KYCAddress) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "t"
	o = append(o, 0x83, 0xa1, 0x74)
	o = msgp.AppendString(o, z.TradeAddress)
	// string "c"
	o = append(o, 0xa1, 0x63)
	o = msgp.AppendString(o, z.Comment)
	// string "v"
	o = append(o, 0xa1, 0x76)
	o = msgp.AppendBool(o, z.Valid)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *KYCAddress) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "t":
			z.TradeAddress, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "TradeAddress")
				return
			}
		case "c":
			z.Comment, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z KYCAddress) Msgsize() (s int) {
	s = 1 + 2 + msgp.StringPrefixSize + len(z.TradeAddress) + 2 + msgp.StringPrefixSize + len(z.Comment) + 2 + msgp.BoolSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *KYCAdminAccount) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "c":
			z.Comment, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z KYCAdminAccount) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "c"
	err = en.Append(0x82, 0xa1, 0x63)
	if err != nil {
		return
	}
	err = en.WriteString(z.Comment)
	if err != nil {
		err = msgp.WrapError(err, "Comment")
		return
	}
	// write "v"
	err = en.Append(0xa1, 0x76)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Valid)
	if err != nil {
		err = msgp.WrapError(err, "Valid")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z KYCAdminAccount) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "c"
	o = append(o, 0x82, 0xa1, 0x63)
	o = msgp.AppendString(o, z.Comment)
	// string "v"
	o = append(o, 0xa1, 0x76)
	o = msgp.AppendBool(o, z.Valid)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *KYCAdminAccount) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "c":
			z.Comment, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z KYCAdminAccount) Msgsize() (s int) {
	s = 1 + 2 + msgp.StringPrefixSize + len(z.Comment) + 2 + msgp.BoolSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *KYCOperatorAccount) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "c":
			z.Comment, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z KYCOperatorAccount) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "c"
	err = en.Append(0x82, 0xa1, 0x63)
	if err != nil {
		return
	}
	err = en.WriteString(z.Comment)
	if err != nil {
		err = msgp.WrapError(err, "Comment")
		return
	}
	// write "v"
	err = en.Append(0xa1, 0x76)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Valid)
	if err != nil {
		err = msgp.WrapError(err, "Valid")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z KYCOperatorAccount) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "c"
	o = append(o, 0x82, 0xa1, 0x63)
	o = msgp.AppendString(o, z.Comment)
	// string "v"
	o = append(o, 0xa1, 0x76)
	o = msgp.AppendBool(o, z.Valid)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *KYCOperatorAccount) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "c":
			z.Comment, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Comment")
				return
			}
		case "v":
			z.Valid, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z KYCOperatorAccount) Msgsize() (s int) {
	s = 1 + 2 + msgp.StringPrefixSize + len(z.Comment) + 2 + msgp.BoolSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *KYCStatus) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "s":
			z.Status, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Status")
				return
			}
		case "v":
			z.Valid, err = dc.ReadBool()
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z KYCStatus) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 2
	// write "s"
	err = en.Append(0x82, 0xa1, 0x73)
	if err != nil {
		return
	}
	err = en.WriteString(z.Status)
	if err != nil {
		err = msgp.WrapError(err, "Status")
		return
	}
	// write "v"
	err = en.Append(0xa1, 0x76)
	if err != nil {
		return
	}
	err = en.WriteBool(z.Valid)
	if err != nil {
		err = msgp.WrapError(err, "Valid")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z KYCStatus) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 2
	// string "s"
	o = append(o, 0x82, 0xa1, 0x73)
	o = msgp.AppendString(o, z.Status)
	// string "v"
	o = append(o, 0xa1, 0x76)
	o = msgp.AppendBool(o, z.Valid)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *KYCStatus) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "s":
			z.Status, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Status")
				return
			}
		case "v":
			z.Valid, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Valid")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z KYCStatus) Msgsize() (s int) {
	s = 1 + 2 + msgp.StringPrefixSize + len(z.Status) + 2 + msgp.BoolSize
	return
}
