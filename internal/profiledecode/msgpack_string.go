package profiledecode

import "encoding/binary"

func decodeMsgpackString(envelope []byte) ([]byte, error) {
	if len(envelope) == 0 {
		return nil, newDecodeError(ERR_IMPORT_LENGTH_MISMATCH, nil)
	}

	headerSize := 1
	var declaredLength uint64
	switch tag := envelope[0]; {
	case tag >= 0xa0 && tag <= 0xbf:
		declaredLength = uint64(tag & 0x1f)
	case tag == 0xd9:
		headerSize = 2
		if len(envelope) < headerSize {
			return nil, newDecodeError(ERR_IMPORT_LENGTH_MISMATCH, nil)
		}
		declaredLength = uint64(envelope[1])
	case tag == 0xda:
		headerSize = 3
		if len(envelope) < headerSize {
			return nil, newDecodeError(ERR_IMPORT_LENGTH_MISMATCH, nil)
		}
		declaredLength = uint64(binary.BigEndian.Uint16(envelope[1:headerSize]))
	case tag == 0xdb:
		headerSize = 5
		if len(envelope) < headerSize {
			return nil, newDecodeError(ERR_IMPORT_LENGTH_MISMATCH, nil)
		}
		declaredLength = uint64(binary.BigEndian.Uint32(envelope[1:headerSize]))
	default:
		return nil, newDecodeError(ERR_IMPORT_MSGPACK_TYPE, nil)
	}

	if declaredLength != uint64(len(envelope)-headerSize) {
		return nil, newDecodeError(ERR_IMPORT_LENGTH_MISMATCH, nil)
	}
	return envelope[headerSize:], nil
}
