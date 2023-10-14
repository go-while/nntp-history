package history

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
	"strconv"
	"strings"
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

func concatInt64(data []int64) []byte {
	strSlice := make([]string, len(data))
	for i, value := range data {
		strSlice[i] = strconv.FormatInt(value, 10)
	}
	return []byte(strings.Join(strSlice, ","))
}

func parseByteToSlice(input []byte) ([]int64, error) {
	parts := bytes.Split(input, []byte(","))
	result := make([]int64, len(parts))

	for i, part := range parts {
		value, err := strconv.ParseInt(string(part), 10, 64)
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

/*
func EncodeOffsets(offsets []int64, char string, bucket string, key string, src string, his *HISTORY) ([]byte, error) {
	return concatInt64(offsets), nil

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
			if _, err := DecodeOffsets(buf.Bytes(), char, bucket, key, src+":test:gobEncodeOffsets", his); err != nil {
				log.Printf("ERROR TEST gobEncodeOffsets [%s|%s] key='%s' offsets='%#v' err='%v'", char, bucket, key, offsets, err)
				return nil, err
			}
		}
		//var ret []byte
		if B64GOB {
			ret = []byte(base64.StdEncoding.EncodeToString(buf.Bytes()))
			ret = append(ret, '.')
			logf(DBG_B64_SIZE, "gobEncodeOffsets input=%d output=%d", len(buf.Bytes()), len(ret))
		} else {
			ret = buf.Bytes()
		}

		return ret, nil

} // end func EncodeOffsets
*/

func DecodeOffsets(encodedData []byte, char string, bucket string, key string, src string, his *HISTORY) ([]int64, error) {
	decodedOffsets, err := parseByteToSlice(encodedData)
	if err != nil {
		log.Printf("ERROR gobDecodeOffsets parseByteToSlice err='%v' encodedData='%s'", err, string(encodedData))
		return nil, err
	}
	return decodedOffsets, nil
	/*
		//var decoder *gob.Decoder

		if B64GOB {
			var b64decoded []byte
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
			log.Printf("ERROR E9 gobDecodeOffsets err='%v'")
			//log.Printf("ERROR E9 gobDecodeOffsets [%s|%s] key='%s' b64decoded='%s' encodedData='%#v' len=%d gobOffsets='%#v' err='%v' src=%s",
			//	char, bucket, key, b64decoded, encodedData, len(encodedData), decodedOffsets, err, src)
			//if err == io.EOF {
			// TODO something is bugging with unexpected EOF, sometimes. 1 in 500M maybe, or not.
			//	//continue
			//}
			return nil, err
		}
		return decodedOffsets, nil
	*/
} // end func DecodeOffsets

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

/*
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
	hash.Write([]byte(data))
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
*/

func FNV32S(data string) (key string) {
	hash := fnv.New32()
	hash.Write([]byte(data))
	key = fmt.Sprintf("%d", hash.Sum32())
	return
} // end func FNV32S

func FNV32aS(data string) (key string) {
	hash := fnv.New32a()
	hash.Write([]byte(data))
	key = fmt.Sprintf("%d", hash.Sum32())
	return
} // end func FNV32aS

func FNV64S(data string) (key string) {
	hash := fnv.New64()
	hash.Write([]byte(data))
	key = fmt.Sprintf("%d", hash.Sum64())
	return
} // end func FNV64S

func FNV64aS(data string) (key string) {
	hash := fnv.New64a()
	hash.Write([]byte(data))
	key = fmt.Sprintf("%d", hash.Sum64())
	return
} // end func FNV64aS
