package profiledecode

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/ulikunitz/xz/lzma"
)

const lzmaHeaderSize = 13

type lzmaHeader struct {
	dictionarySize uint32
	outputSize     uint64
}

func hasLZMAHeader(source []byte) bool {
	if normalized, err := normalizeOuterText(string(source)); err == nil {
		if _, err := detectOuter(normalized); err == nil {
			return false
		}
	}
	_, err := parseLZMAHeader(source)
	return err == nil
}

func parseLZMAHeader(source []byte) (lzmaHeader, error) {
	if len(source) < lzmaHeaderSize || source[0] >= 9*5*5 {
		return lzmaHeader{}, newDecodeError(ERR_IMPORT_LZMA_HEADER, nil)
	}

	return lzmaHeader{
		dictionarySize: binary.LittleEndian.Uint32(source[1:5]),
		outputSize:     binary.LittleEndian.Uint64(source[5:13]),
	}, nil
}

func decodeLZMA(compressed []byte) ([]byte, error) {
	if len(compressed) > maxCompressedSize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}

	header, err := parseLZMAHeader(compressed)
	if err != nil {
		return nil, err
	}
	if header.dictionarySize > maxDictionarySize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}
	if header.outputSize != math.MaxUint64 && header.outputSize > maxOutputSize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}

	source := bytes.NewReader(compressed)
	reader, err := (lzma.ReaderConfig{DictCap: maxDictionarySize}).NewReader(source)
	if err != nil {
		return nil, newDecodeError(ERR_IMPORT_LZMA_CORRUPT, err)
	}
	output, err := io.ReadAll(io.LimitReader(reader, int64(maxOutputSize)+1))
	if err != nil {
		return nil, newDecodeError(ERR_IMPORT_LZMA_CORRUPT, err)
	}
	if len(output) > maxOutputSize {
		return nil, newDecodeError(ERR_IMPORT_LIMIT_EXCEEDED, nil)
	}
	if header.outputSize != math.MaxUint64 && uint64(len(output)) != header.outputSize {
		return nil, newDecodeError(ERR_IMPORT_LZMA_CORRUPT, nil)
	}
	if source.Len() != 0 {
		return nil, newDecodeError(ERR_IMPORT_LZMA_CORRUPT, nil)
	}
	return output, nil
}
