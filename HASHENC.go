package history

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"hash/crc32"
	//"encoding/hex"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"strconv"
	"strings"
	//"time"
)

var (
	ADDCRC bool = false
)

// idea: create a struct to catch N lastchar of the hash
//
//	place the lastchar(s) with the stored offset (lastchar|offset,lastchar|offset,...) in boltdb with key[1:keylen]
//	so we can ignore offsets not matching the last char when checking if a hash exists in hisDat
func concatInt64(input []int64, output *[]byte) (int, error) {
	if input == nil || len(input) == 0 || output == nil {
		return 0, fmt.Errorf("ERROR concatInt64 io nil")
	}
	strSlice := make([]string, len(input))
	for i, value := range input {
		strSlice[i] = strconv.FormatInt(value, 16) // stores int64 as hex string
		//if ADDCRC {
		//	_, _ = CRC(strSlice[i])
		//}
	}
	// joins ints with , into a []byte
	*output = []byte(strings.Join(strSlice, ","))

	// adds comma to strings EOL
	*output = append(*output, ',')
	//log.Printf("concatInt64 input='%#v' output='%s'", input, *output)
	return len(*output), nil
} // end func concatInt64

func parseByteToSlice(input []byte, result *[]int64) (int, error) {
	if input == nil || result == nil {
		return 0, fmt.Errorf("ERROR parseByteToSlice io nil")
	}
	if input[len(input)-1] != ',' {
		return 0, fmt.Errorf("ERROR parseByteToSlice EOL!=','")
	}
	parts := bytes.Split(input, []byte(","))
transform:
	for _, part := range parts {
		if len(part) == 0 {
			//log.Printf("parseByteToSlice ignored i=%d part='%s'", i, part)
			continue transform
		}
		value, err := strconv.ParseInt(string(part), 16, 64) // reads hex
		if err != nil {
			log.Printf("ERROR parseByteToSlice err='%v'", err)
			return 0, err
		}
		//log.Printf("parseByteToSlice i=%d part='%s'=>value=%d result='%#v'", i, string(part), value, result)
		*result = append(*result, value)
	}
	//log.Printf("parseByteToSlice input=%s result='%#v'", string(input), parts, result)
	return len(*result), nil
} // end func parseByteToSlice

func gobEncodeHeader(iobuf *[]byte, settings *HistorySettings) (int, error) {
	if iobuf == nil || settings == nil {
		log.Printf("ERROR gobEncodeHeader iobuf or settings nil")
		os.Exit(1)
	}
	ZEROPADLEN := 254 // later adds 1 more: LF
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		log.Printf("ERROR gobEncodeHeader Encode err='%v'", err)
		return 0, err
	}
	b64str := base64.StdEncoding.EncodeToString(buf.Bytes())
	b64strZeroPadded := ZeroPad(b64str, ZEROPADLEN)
	*iobuf = []byte(b64strZeroPadded)
	leniobuf := len(*iobuf)
	log.Printf("gobEncodeHeader b64str='%s' b64strZeroPadded='%s' lenio=%d", b64str, b64strZeroPadded, leniobuf)
	return leniobuf, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData *[]byte, retSettings *HistorySettings) error {
	if encodedData == nil || retSettings == nil {
		return fmt.Errorf("ERROR gobDecodeHeader io=nil")
	}
	b64decodedString, err := base64.StdEncoding.DecodeString(RemoveZeroPad(string(*encodedData)))
	if err != nil {
		return fmt.Errorf("ERROR gobDecodeHeader base64decode err='%v'", err)
	}
	//decoder := gob.NewDecoder(bytes.NewBuffer([]byte(b64decodedString)))
	//err = decoder.Decode(&retSettings)
	err = gob.NewDecoder(bytes.NewBuffer([]byte(b64decodedString))).Decode(&retSettings)
	if err != nil {
		return fmt.Errorf("ERROR gobDecodeHeader Decode err='%v'", err)
	}
	return nil
} // end func gobDecodeHeader

func ZeroPad(input string, length int) string {
	if len(input) < length {
		padding := strings.Repeat("\x00", length-len(input))
		return padding + input
	}
	return input
} // end func ZeroPad

func RemoveZeroPad(input string) string {
	return strings.Replace(input, "\x00", "", -1)
} // RemoveZeroPad

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

func CRC(input string) (string, error) {
	// Convert the string to bytes.
	hash := crc32.NewIEEE()
	_, err := hash.Write([]byte(input))
	if err != nil {
		return "", err
	}
	crc := hash.Sum32()
	checksumStr := strconv.FormatInt(int64(crc), 16) // as hex
	log.Printf("CRC input='%s' output='%s' crc=%d", input, checksumStr, crc)
	return checksumStr, nil
} // end func CRC
