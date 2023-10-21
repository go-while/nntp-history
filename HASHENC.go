package history

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"hash/crc32"
	//"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	//"time"
	"crypto/rand"
	"math/big"
)

var (
	// set HEX true: converts offset into hex strings to store in bbolt
	// dont change later once db is initialized!
	HEX bool = true

	//ADDCRC bool = false
)

type Offsets struct {
	offsets []int64
	hashLCR map[int64]string
}

func concatInt64(input []int64, output *[]byte) (int, error) {
	if input == nil || len(input) == 0 || output == nil {
		return 0, fmt.Errorf("ERROR concatInt64 io nil")
	}
	strSlice := make([]string, len(input))
	for i, value := range input {
		if HEX {
			// stores int64 as hex string
			strSlice[i] = strconv.FormatInt(value, 16)
		} else {
			// stores int64 as digit string
			strSlice[i] = strconv.FormatInt(value, 10)
		}
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
		if HEX {
			// reads hex
			if value, err := strconv.ParseInt(string(part), 16, 64); err != nil {
				log.Printf("ERROR parseByteToSlice HEX err='%v'", err)
				return 0, err
			} else {
				*result = append(*result, value)
			}
		} else {
			// reads digits
			if value, err := strconv.ParseInt(string(part), 10, 64); err != nil {
				log.Printf("ERROR parseByteToSlice DIG err='%v'", err)
				return 0, err
			} else {
				*result = append(*result, value)
			}
		}
		//log.Printf("parseByteToSlice i=%d part='%s'=>value=%d result='%#v'", i, string(part), value, result)

	}
	//log.Printf("parseByteToSlice input=%s result='%#v'", string(input), parts, result)
	return len(*result), nil
} // end func parseByteToSlice

func gobEncodeHeader(iobuf *[]byte, settings *HistorySettings) (int, error) {
	if iobuf == nil || settings == nil {
		log.Printf("ERROR gobEncodeHeader iobuf or settings nil")
		os.Exit(1)
	}
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(settings)
	if err != nil {
		log.Printf("ERROR gobEncodeHeader Encode err='%v'", err)
		return 0, err
	}
	b64str := base64.StdEncoding.EncodeToString(buf.Bytes())
	NullPad(&b64str, ZEROPADLEN)
	*iobuf = []byte(b64str)
	leniobuf := len(*iobuf)
	log.Printf("gobEncodeHeader\n b64str='%s'=%d lenio=%d\n settings='%#v'", b64str, len(buf.Bytes()), leniobuf, settings)
	return leniobuf, nil
} // end func gobEncodeHeader

func gobDecodeHeader(encodedData []byte, retSettings *HistorySettings) error {
	if encodedData == nil || retSettings == nil {
		return fmt.Errorf("ERROR gobDecodeHeader io=nil")
	}
	if b64decodedString, err := base64.StdEncoding.DecodeString(RemoveNullPad(string(encodedData))); err == nil {
		//decode header into supplied retSettings pointer
		if err := gob.NewDecoder(bytes.NewBuffer([]byte(b64decodedString))).Decode(&retSettings); err != nil {
			return fmt.Errorf("ERROR gobDecodeHeader Decode err='%v'", err)
		}
	} else {
		return fmt.Errorf("ERROR gobDecodeHeader base64decode err='%v'", err)
	}
	return nil
} // end func gobDecodeHeader

func LeftPad(input *string, length int) {
	if len(*input) < length {
		padding := strings.Repeat("\x00", length-len(*input))
		*input = padding + *input
	}
} // end func LeftPad

func NullPad(input *string, length int) {
	if len(*input) < length {
		padding := strings.Repeat("\x00", length-len(*input))
		*input = *input + padding
	}
} // end func NullPad

func RemoveNullPad(input string) string {
	return strings.Replace(input, "\x00", "", -1)
} // RemoveZeroPad

func CRC(input string) string {
	hash := crc32.NewIEEE()
	_, err := hash.Write([]byte(input))
	if err != nil {
		return ""
	}
	crc := hash.Sum32()
	checksumStr := strconv.FormatInt(int64(crc), 16) // as hex
	//log.Printf("CRC input='%s' output='%s'", input, checksumStr)
	return checksumStr
} // end func CRC

func generateRandomInt(min, max int) (int, error) {
	if min > max {
		return 0, fmt.Errorf("Invalid range: min > max")
	}

	// Calculate the range size
	rangeSize := big.NewInt(int64(max - min + 1))

	// Generate a random integer within the specified range
	randomNumber, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return 0, err
	}

	// Add min to shift the range to the desired values
	randomNumber.Add(randomNumber, big.NewInt(int64(min)))

	// Convert the *big.Int to an int
	randomInt := int(randomNumber.Int64())

	return randomInt, nil
}
