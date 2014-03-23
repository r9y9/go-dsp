package wav

import (
	"encoding/binary"
	"os"
)

func (w *Wav) GetMonoData() []float64 {
	y := make([]float64, len(w.Data))
	if int(w.NumChannels) == 1 {
		for i, val := range w.Data {
			y[i] = float64(val[0])
		}
		return y
	}
	for i, val := range w.Data {
		y[i] = float64(val[0]+val[1]) / 2.0
	}
	return y
}

func WriteMono(filename string, data []float64, sampleRate uint32) error {
	bitsPerSample := 16
	channels := 1

	outFile := &File{
		sampleRate,
		uint16(bitsPerSample),
		uint16(channels),
	}

	// []int to []bytes (assuming 16-bit samples)
	bytes := make([]byte, 2*len(data))
	for i, val := range data {
		start := i * 2
		binary.LittleEndian.PutUint16(bytes[start:start+2], uint16(val))
	}

	ofile, oerr := os.Create(filename)
	if oerr != nil {
		return oerr
	}

	err := outFile.WriteData(ofile, bytes)

	if err != nil {
		return err
	}

	return nil
}
