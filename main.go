package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/gotranspile/g722"
	"github.com/pd0mz/go-g711"
	// "github.com/zaf/g711"
)

const (
	inputSampleRate  = 16000
	outputSampleRate = 8000
)

const (
	SIGN_BIT   = 0x80 // Sign bit for a A-law byte.
	QUANT_MASK = 0xf  // Quantization field mask.
	NSEGS      = 8    // Number of A-law segments.
	SEG_SHIFT  = 4    // Left shift for segment number.
	SEG_MASK   = 0x70 // Segment field mask.
)

var segEnd = [8]int16{
	0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF, 0x3FFF, 0x7FFF,
}

func search(val int, table []int16, size int) int {
	for i := 0; i < size; i++ {
		if val <= int(table[i]) {
			return i
		}
	}
	return size
}

// Linear2Alaw 将16位线性PCM值转换为8位A-law
// 输入参数范围：-32768~32767
// 返回8位无符号整数
func Linear2Alaw(pcmVal int16) byte {
	var mask int
	var seg int
	var aval byte

	// 右移3位，因为采样值是16bit，而A-law是13bit，存储在高13位上，低3位被舍弃
	pcmVal >>= 3

	if pcmVal >= 0 {
		mask = 0xD5 // sign (7th) bit = 1 二进制的11010101
	} else {
		mask = 0x55          // sign bit = 0 二进制的01010101
		pcmVal = -pcmVal - 1 // 负数转换为正数计算
	}

	// Convert the scaled magnitude to segment number.
	seg = search(int(pcmVal), segEnd[:], 8) // 返回pcmVal属于哪个分段

	// Combine the sign, segment, and quantization bits.
	if seg >= 8 { // out of range, return maximum value.
		return byte(0x7F ^ mask)
	} else {
		aval = byte(seg << SEG_SHIFT)
		// aval为每一段的偏移，分段量化后的数据需要加上该偏移（aval）
		// 分段量化
		// 量化方法： (pcm_val-分段值)，然后取有效的高4位 （0分段例外）
		// 比如 pcm_val = 0x7000 ，那么seg=7 ，第7段的范围是0x4000~0x7FFF
		// ，段偏移aval=7<<4=0x7F 0x7000-0x4000=0x3000
		// ，然后取有效的高4位，即右移10(seg+3)，0x3000>>10=0xC
		// 上一步等效为：(0x7000>>10)&0xF=0xC 。也就是： (pcm_val >> (seg + 3)) & QUANT_MASK
		// 然后加上段偏移 0x7F(aval) ，加法等效于或运算，即 |aval

		if seg < 2 {
			aval |= byte((pcmVal >> 4) & QUANT_MASK) // 0、1段折线的斜率一样
		} else {
			aval |= byte((pcmVal >> (seg + 3)) & QUANT_MASK)
		}
		return aval ^ byte(mask) // 异或0x55，目的是尽量避免出现连续的0，或连续的1，提高传输过程的可靠性
	}
}

func decodeWithFlags(data []byte, flags g722.Flags) []int16 {
	return g722.Decode(data, g722.RateDefault, flags)
}

func pcm16bytes(amp []int16) []byte {
	out := make([]byte, 2*len(amp))
	for i, v := range amp {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(v))
	}
	return out
}

// func ALawEncodeSample(s int16) uint8 {
// 	if s >= 0 {
// 		return aLawCompressTable[s>>4]
// 	}
// 	return 0x7f & aLawCompressTable[-s>>4]
// }

// func ALawEncode(in []int16) []uint8 {
// 	if in == nil {
// 		return nil
// 	}
// 	out := make([]uint8, len(in))
// 	for i, s := range in {
// 		out[i] = ALawEncodeSample(s)
// 	}
// 	return out
// }

func pcm16ToAlaw(samples []int16, method int) []byte {

	buf := bytes.NewBuffer(nil)
	var downsampleCounter int
	for _, sample := range samples {
		// 简单的降采样：每两个样本取一个
		downsampleCounter++
		if downsampleCounter%2 == 0 {
			// encodedSample := encodeAlaw(sample)
			var encodedSample byte
			switch method {
			case 0:
				encodedSample = Linear2Alaw(sample)
			case 1:
				encodedSample = g711.ALawEncodeSample(sample)
			}

			err := buf.WriteByte(encodedSample)
			if err != nil {
				fmt.Println("Error writing output:", err)
				return nil
			}
		}
	}
	return buf.Bytes()
}

func main() {
	data, _ := os.ReadFile("input.g722")
	pcm := decodeWithFlags(data, 0)

	filePcm, _ := os.OpenFile("output.pcm", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	defer filePcm.Close()
	filePcm.Write(pcm16bytes(pcm))

	fileG711, _ := os.OpenFile("output0.g711", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	defer fileG711.Close() // 确保在函数结束时关闭文件
	fileG711.Write(pcm16ToAlaw(pcm, 0))

	fileG711_1, _ := os.OpenFile("output1.g711", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	defer fileG711_1.Close() // 确保在函数结束时关闭文件
	fileG711_1.Write(pcm16ToAlaw(pcm, 1))
}
