package cscall

// G.711 A-law 编解码 (ITU-T G.711)
// 用于 Linphone 发送 PT=1 (PCMA/A-law) 时的编解码

// LinearToAlaw 将 16-bit 线性 PCM 样本转换为 8-bit A-law
func LinearToAlaw(sample int16) byte {
	const (
		clip = 32635
	)

	sign := sample < 0
	if sign {
		sample = -sample
	}
	if sample > clip {
		sample = clip
	}

	// A-law 压缩
	var alawByte byte
	if sample < 256 {
		// 线性段 (0-255 → 0-127 in steps of 2)
		alawByte = byte(sample >> 4) // 0-15
	} else {
		// 对数段
		exponent := 0
		tmp := sample >> 8
		for tmp > 0 {
			tmp >>= 1
			exponent++
		}
		if exponent > 7 {
			exponent = 7
		}
		// 计算尾数
		mantissa := (sample >> (exponent + 3)) & 0x0F
		alawByte = byte(exponent<<4) | byte(mantissa)
	}

	if sign {
		alawByte ^= 0x80
	}

	// A-law 按位取反（与 μ-law 不同，A-law 在传输时也取反）
	return ^alawByte
}

// AlawToLinear 将 8-bit A-law 转换为 16-bit 线性 PCM 样本
func AlawToLinear(alaw byte) int16 {
	alaw = ^alaw
	sign := alaw & 0x80
	exponent := (alaw >> 4) & 0x07
	mantissa := alaw & 0x0F

	var sample int
	if exponent == 0 {
		// 线性段
		sample = int(mantissa) << 4
	} else {
		// 对数段
		sample = (int(mantissa) << 4) + 0x108
		sample <<= exponent - 1
	}

	result := int16(sample)
	if sign != 0 {
		result = -result
	}
	return result
}

// EncodePCMToAlaw 批量将 PCM S16_LE 数据编码为 A-law
func EncodePCMToAlaw(pcm []byte) []byte {
	numSamples := len(pcm) / 2
	out := make([]byte, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(pcm[i*2]) | int16(pcm[i*2+1])<<8
		out[i] = LinearToAlaw(sample)
	}
	return out
}

// DecodeAlawToPCM 批量将 A-law 数据解码为 PCM S16_LE
func DecodeAlawToPCM(alaw []byte) []byte {
	out := make([]byte, len(alaw)*2)
	for i, a := range alaw {
		sample := AlawToLinear(a)
		out[i*2] = byte(sample)
		out[i*2+1] = byte(sample >> 8)
	}
	return out
}
