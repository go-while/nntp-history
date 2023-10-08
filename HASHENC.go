package history

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
)

func FNV32(data *string) (*string, *uint32) {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV32

func FNV64(data *string) (*string, *uint64) {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV64

func FNV32S(data *string) *string {
	hash := fnv.New32()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32S

func FNV32aS(data *string) *string {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV32aS

func FNV64S(data *string) *string {
	hash := fnv.New64()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64S

func FNV64aS(data *string) *string {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	s := fmt.Sprintf("%d", d)
	return &s
} // end func FNV64aS

func gobEncodeHeader(settings *HistorySettings) (*[]byte, error) {
	gob.Register(HistorySettings{})
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		log.Printf("ERROR gobEncodeHeader Encode err='%v'", err)
		return nil, err
	}
	encodedData := buf.Bytes()
	//return &encodedData, nil
	b64 := []byte(base64.StdEncoding.EncodeToString(encodedData))
	return &b64, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData []byte) (*HistorySettings, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		log.Printf("ERROR gobDecodeHeader base64decode err='%v'", err)
		return nil, err
	}
	buf := bytes.NewBuffer([]byte(decoded))
	//buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	settings := &HistorySettings{}
	err = decoder.Decode(settings)
	if err != nil {
		log.Printf("ERROR gobDecodeHeader Decode err='%v'", err)
		return nil, err
	}
	return settings, nil
} // end func gobDecodeHeader

func gobEncodeOffsets(offsets []int64, src string) ([]byte, error) {
	gob.Register(GOBOFFSETS{})
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	gobOffsets := &GOBOFFSETS{Offsets: offsets}
	err := encoder.Encode(*gobOffsets)
	if err != nil {
		log.Printf("ERROR gobEncodeOffsets offsets='%#v' err='%v'", offsets, err)
		return nil, err
	}
	encodedData := buf.Bytes()
	// costly check try decode encodedData
	//if _, err := gobDecodeOffsets(encodedData, src+":test:gobEncodeOffsets"); err != nil {
	//	return nil, err
	//}
	return encodedData, nil
} // end func gobEncodeOffsets

func gobDecodeOffsets(encodedData []byte, src string) (*[]int64, error) {
	buf := bytes.NewBuffer(encodedData)
	decoder := gob.NewDecoder(buf)
	//var decodedOffsets []int64
	gobOffsets := &GOBOFFSETS{}
	err := decoder.Decode(gobOffsets)
	if err != nil {
		log.Printf("ERROR gobDecodeOffsets encodedData='%#v' len=%d gobOffsets='%#v' err='%v' src=%s", encodedData, len(encodedData), gobOffsets, err, src)
		return nil, err
	}
	return &gobOffsets.Offsets, nil
	//return &decodedOffsets, nil
} // end func gobDecodeOffsets
