package history

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
)

var (
	DBG_GOB_TEST bool = false // costly check: test decodes gob encoded data after encoding
	DBG_B64_SIZE bool = false // spams every b64 encoded input:output size
	B64GOB       bool = false
)

type GOBENC struct {
	GobEncoder *gob.Encoder
}

type GOBDEC struct {
	GobDecoder *gob.Decoder
}

func createGobDecoder(encodedData []byte) (*gob.Decoder, error) {
	/*
		// Create a reusable gob decoder
		decoder, err := createGobDecoder(encodedData)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		*
		*
		// Decode data using the reusable decoder
		var decodedData T
		err = decoder.Decode(&decodedData)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	*/

	// Create a buffer with the encoded data
	buffer := bytes.NewBuffer(encodedData)

	// Initialize a new decoder with the buffer
	decoder := gob.NewDecoder(buffer)

	return decoder, nil
}

func gobEncodeOffsets(offsets []int64, char string, bucket string, key string, src string, his *HISTORY) ([]byte, error) {
	//err = his.GobDecoder[*char].GobDecoder.Decode(&decodedOffsets) //gob.NewDecoder(bytes.NewBuffer(encodedData))
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	//err := encoder.Encode(GOBOFFSETS{Offsets: offsets})
	//gobOffsets := GOBOFFSETS{Offsets: offsets}
	err := encoder.Encode(offsets)
	if err != nil {
		log.Printf("ERROR gobEncodeOffsets [%s|%s] key='%s' offsets='%#v' err='%v'", char, bucket, key, offsets, err)
		return nil, err
	}
	if DBG_GOB_TEST {
		// test costly check retry decode gob encodedData
		if _, err := gobDecodeOffsets(buf.Bytes(), char, bucket, key, src+":test:gobEncodeOffsets", his); err != nil {
			log.Printf("ERROR TEST gobEncodeOffsets [%s|%s] key='%s' offsets='%#v' err='%v'", char, bucket, key, offsets, err)
			return nil, err
		}
	}
	var ret []byte
	if B64GOB {
		ret = []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
		ret = append(ret, '.')
		logf(DBG_B64_SIZE, "gobEncodeOffsets input=%d output=%d", len(buf.Bytes()), len(ret))
	} else {
		ret = buf.Bytes()
	}

	return ret, nil
} // end func gobEncodeOffsets

func gobDecodeOffsets(encodedData []byte, char string, bucket string, key string, src string, his *HISTORY) (*[]int64, error) {
	var decodedOffsets []int64
	//var decoder *gob.Decoder
	var err error
	var b64decoded []byte
	if B64GOB {
		len_enc := len(encodedData)
		if encodedData[len_enc-1] != '.' {
			err := fmt.Errorf("ERROR gobDecodeOffsets [%s|%s] key='%s' E0 encodedData='%s'", char, bucket, key, encodedData)
			return nil, err
		}
		b64decoded, err = base64.StdEncoding.DecodeString(string(encodedData[:len_enc-1]))
		len_b64 := len(b64decoded)
		if err != nil {
			log.Printf("ERROR gobDecodeOffsets [%s|%s] key='%s' base64decode E1 b64decoded=%d='%s' err='%v'", char, bucket, key, len_b64, b64decoded, err)
			return nil, err
		}
		if len_b64 <= 1 {
			err = fmt.Errorf("ERROR gobDecodeOffsets [%s|%s] key='%s' E2 b64decoded=%d='%s' encodedData=%d='%s'", char, bucket, key, len_b64, b64decoded, len(encodedData), string(encodedData))
			return nil, err
		}
		buf := bytes.NewBuffer(b64decoded)
		decoder := gob.NewDecoder(buf)
		err = decoder.Decode(&decodedOffsets)
	} else {
		buf := bytes.NewBuffer(encodedData)
		decoder := gob.NewDecoder(buf)
		err = decoder.Decode(&decodedOffsets)
	}
	if err != nil {
		log.Printf("ERROR E9 gobDecodeOffsets [%s|%s] key='%s' b64decoded='%s' encodedData='%#v' len=%d gobOffsets='%#v' err='%v' src=%s",
			char, bucket, key, b64decoded, encodedData, len(encodedData), decodedOffsets, err, src)
		//if err == io.EOF {
		// TODO something is bugging with unexpected EOF, sometimes. 1 in 500M maybe, or not.
		//	//continue
		//}
		return nil, err
	}
	return &decodedOffsets, nil
} // end func gobDecodeOffsets

func gobEncodeHeader(settings *HistorySettings) (*[]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		log.Printf("ERROR gobEncodeHeader Encode err='%v'", err)
		return nil, err
	}
	b64 := []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
	return &b64, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData []byte) (*HistorySettings, error) {
	b64decodedString, err := base64.StdEncoding.DecodeString(string(encodedData))
	if err != nil {
		log.Printf("ERROR gobDecodeHeader base64decode err='%v'", err)
		return nil, err
	}
	buf := bytes.NewBuffer([]byte(b64decodedString))
	decoder := gob.NewDecoder(buf)
	settings := &HistorySettings{}
	err = decoder.Decode(settings)
	if err != nil {
		log.Printf("ERROR gobDecodeHeader Decode err='%v'", err)
		return nil, err
	}
	return settings, nil
} // end func gobDecodeHeader

func FNV32(data *string) (*string, *uint32) {
	hash := fnv.New32()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV32

func FNV64(data *string) (*string, *uint64) {
	hash := fnv.New64()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV64

func FNV32a(data *string) (*string, *uint32) {
	hash := fnv.New32a()
	hash.Write([]byte(*data))
	d := hash.Sum32()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV32a

func FNV64a(data *string) (*string, *uint64) {
	hash := fnv.New64a()
	hash.Write([]byte(*data))
	d := hash.Sum64()
	hex := fmt.Sprintf("0x%08x", d)
	return &hex, &d
} // end func FNV64a

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
