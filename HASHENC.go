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
	//"time"
)

func concatInt64(data []int64) []byte {
	strSlice := make([]string, len(data))
	for i, value := range data {
		//strSlice[i] = fmt.Sprintf("%x", strconv.FormatInt(value, 10)) // stores as hex
		strSlice[i] = strconv.FormatInt(value, 10) // stores digits
	}
	retdata := []byte(strings.Join(strSlice, ","))
	retdata = append(retdata, ',') // adds comma to strings EOL
	return retdata
}

func parseByteToSlice(input []byte) (result []int64, err error) {
	if input[len(input)-1] != ',' {
		return nil, fmt.Errorf("ERROR parseByteToSlice EOL!=','")
	}
	parts := bytes.Split(input, []byte(","))
transform:
	for _, part := range parts {
		if len(part) == 0 {
			//log.Printf("parseByteToSlice ignored i=%d part='%s'", i, part)
			continue transform
		}
		value, err := strconv.ParseInt(string(part), 10, 64)
		if err != nil {
			log.Printf("ERROR parseByteToSlice err='%v'")
			return nil, err
		}
		//log.Printf("parseByteToSlice i=%d part='%s'=>value=%d result='%#v'", i, string(part), value, result)
		result = append(result, value)
	}
	//log.Printf("parseByteToSlice input=%s result='%#v'", string(input), parts, result)
	return result, nil
} // end func parseByteToSlice

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
