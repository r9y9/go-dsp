/*
 * Copyright (c) 2012 Matt Jibson <matt.jibson@gmail.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

// Package wav provides support for the WAV file format.
package wav

import (
	"errors"
	"io"
	"io/ioutil"
	"strings"
)

const (
	RIFFMarkerOffset = 0
	WAVEMarkerOffset = 8
	FMTMarkerOffset  = 12

	AudioFormatOffset   = 20
	NumChannelsOffset   = 22
	SampleRateOffset    = 24
	ByteRateOffset      = 28
	BlockAlignOffset    = 32
	BitsPerSampleOffset = 34
)

type WavHeader struct {
	AudioFormat      uint16
	NumChannels      uint16
	SampleRate       uint32
	ByteRate         uint32
	BlockAlign       uint16
	BitsPerSample    uint16
	ChunkSize        uint32
	NumSamples       int
	DataMarkerOffset int
}

type Wav struct {
	WavHeader

	// The Data corresponding to BitsPerSample is populated, indexed by sample.
	Data8  [][]uint8
	Data16 [][]int16

	// Data is always populated, indexed by sample. It is a copy of DataXX.
	Data [][]int
}

type StreamedWav struct {
	WavHeader
	io.Reader
}

//Scans the file for presence of "data"
func getDataMarkerOffset(filedata []byte) int {
	stringdata := string(filedata)
	if !strings.Contains(stringdata, "data") {
		return -1
	}
	index := strings.Index(stringdata, "data")
	return index
}

func checkHeader(header []byte, datamarkeroffset int) error {
	if string(header[0:4]) != "RIFF" {
		return errors.New("wav: Header does not contain 'RIFF'")
	}
	if string(header[8:12]) != "WAVE" {
		return errors.New("wav: Header does not contain 'WAVE'")
	}
	if string(header[12:16]) != "fmt " {
		return errors.New("wav: Header does not contain 'fmt'")
	}
	if string(header[datamarkeroffset:datamarkeroffset+4]) != "data" {
		return errors.New("wav: Header does not contain 'data'")
	}

	return nil
}

func (wavHeader *WavHeader) setupWithHeaderData(header []byte) (err error) {
	if err = checkHeader(header, wavHeader.DataMarkerOffset); err != nil {
		return
	}

	wavHeader.AudioFormat = bLEtoUint16(header, AudioFormatOffset)
	wavHeader.NumChannels = bLEtoUint16(header, NumChannelsOffset)
	wavHeader.SampleRate = bLEtoUint32(header, SampleRateOffset)
	wavHeader.ByteRate = bLEtoUint32(header, ByteRateOffset)
	wavHeader.BlockAlign = bLEtoUint16(header, BlockAlignOffset)
	wavHeader.BitsPerSample = bLEtoUint16(header, BitsPerSampleOffset)
	wavHeader.ChunkSize = bLEtoUint32(header, wavHeader.DataMarkerOffset+4)
	wavHeader.NumSamples = int(wavHeader.ChunkSize) / int(wavHeader.BlockAlign)

	return
}

// Returns a single sample laid out by channel e.g. [ch0, ch1, ...]
func readSampleFromData(data []byte, sampleIndex int, header WavHeader) (sample []int) {
	sample = make([]int, header.NumChannels)

	for channelIdx := 0; channelIdx < int(header.NumChannels); channelIdx++ {
		if header.BitsPerSample == 8 {
			sample[channelIdx] = int(data[sampleIndex*int(header.NumChannels)+channelIdx])
		} else if header.BitsPerSample == 16 {
			sample[channelIdx] = int(bLEtoInt16(data, 2*sampleIndex*int(header.NumChannels)+channelIdx))
		}
	}

	return
}

// ReadWav reads a wav file.
func ReadWav(r io.Reader) (wav *Wav, err error) {
	if r == nil {
		return nil, errors.New("wav: Invalid Reader")
	}

	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	wav = new(Wav)
	dataMarkerOffset := getDataMarkerOffset(bytes)
	if dataMarkerOffset == -1 {
		err = errors.New("data header not found")
		return nil, err
	}
	wav.DataMarkerOffset = dataMarkerOffset
	err = wav.WavHeader.setupWithHeaderData(bytes)
	if err != nil {
		return nil, err
	}

	data := bytes[dataMarkerOffset+8 : int(wav.ChunkSize)+dataMarkerOffset+8]

	wav.Data = make([][]int, wav.NumSamples)

	if wav.BitsPerSample == 8 {
		wav.Data8 = make([][]uint8, wav.NumSamples)
		for sampleIndex := 0; sampleIndex < wav.NumSamples; sampleIndex++ {
			wav.Data8[sampleIndex] = make([]uint8, wav.NumChannels)
		}

		for i := 0; i < wav.NumSamples; i++ {
			sample := readSampleFromData(data, i, wav.WavHeader)
			wav.Data[i] = sample

			for ch := 0; ch < int(wav.NumChannels); ch++ {
				wav.Data8[i][ch] = uint8(sample[ch])
			}
		}
	} else if wav.BitsPerSample == 16 {
		wav.Data16 = make([][]int16, wav.NumSamples)
		for sampleIndex := 0; sampleIndex < wav.NumSamples; sampleIndex++ {
			wav.Data16[sampleIndex] = make([]int16, wav.NumChannels)
		}

		for i := 0; i < wav.NumSamples; i++ {
			sample := readSampleFromData(data, i, wav.WavHeader)
			wav.Data[i] = sample

			for ch := 0; ch < int(wav.NumChannels); ch++ {
				wav.Data16[i][ch] = int16(sample[ch])
			}
		}
	}

	return
}

// Constructs a StreamedWav which can be read using ReadSamples
func StreamWav(reader io.Reader) (wav *StreamedWav, err error) {
	if reader == nil {
		return nil, errors.New("wav: Invalid Reader")
	}
	stringdata := ""
	headerdataoffset := 0
	for !strings.Contains(stringdata, "data") {
		singlebyte := make([]byte, 1)
		_, readerror := reader.Read(singlebyte)
		if readerror != nil {
			break
		}
		stringdata += string(singlebyte)
		headerdataoffset++
	}

	header := make([]byte, headerdataoffset+8)
	_, err = reader.Read(header)
	if err != nil {
		return nil, err
	}

	wav = new(StreamedWav)
	err = wav.setupWithHeaderData(header)
	if err != nil {
		return nil, err
	}

	wav.Reader = reader

	return
}

// Returns an array of [channelIndex][sampleIndex]
// The number of samples returned may be less than the amount requested
// depending on the amount of data available.
func (wav *StreamedWav) ReadSamples(numSamples int) (samples [][]int, err error) {
	data := make([]byte, numSamples*int(wav.BlockAlign))
	amountRead, err := wav.Reader.Read(data)
	if err != nil {
		return
	}
	if amountRead%int(wav.BlockAlign) != 0 {
		err = errors.New("wav: Read an invalid amount of data")
		return
	}

	numberOfSamplesRead := amountRead / int(wav.BlockAlign)
	samples = make([][]int, numberOfSamplesRead)

	for sampleIndex := 0; sampleIndex < numberOfSamplesRead; sampleIndex++ {
		samples[sampleIndex] = readSampleFromData(data, sampleIndex, wav.WavHeader)
	}

	return
}

// little-endian [4]byte to uint32 conversion
func bLEtoUint32(b []byte, idx int) uint32 {
	return uint32(b[idx+3])<<24 +
		uint32(b[idx+2])<<16 +
		uint32(b[idx+1])<<8 +
		uint32(b[idx])
}

// little-endian [2]byte to uint16 conversion
func bLEtoUint16(b []byte, idx int) uint16 {
	return uint16(b[idx+1])<<8 + uint16(b[idx])
}

func bLEtoInt16(b []byte, idx int) int16 {
	return int16(b[idx+1])<<8 + int16(b[idx])
}
