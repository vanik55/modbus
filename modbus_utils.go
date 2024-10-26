package modbus

import (
	"fmt"
	"sort"
)

func CalculateAddressRange(addresses []int) (lowerAddress, upperAddress int) {
	if len(addresses) == 0 {
		return
	}
	upperAddress = addresses[0]
	lowerAddress = addresses[0]
	for _, value := range addresses {
		if upperAddress < value {
			upperAddress = value
		}
		if lowerAddress > value {
			lowerAddress = value
		}
	}
	return
}

func CalculateAddressRangeMulti(addresses []int) [][2]int {
	if len(addresses) == 0 {
		return nil
	}

	// Sort the addresses
	sort.Ints(addresses)

	var ranges [][2]int
	lower := addresses[0]
	upper := addresses[0]

	for i := 1; i < len(addresses); i++ {
		if addresses[i] == upper+1 {
			upper = addresses[i]
		} else {
			ranges = append(ranges, [2]int{lower, upper})
			lower = addresses[i]
			upper = addresses[i]
		}
	}

	// Append the last range
	ranges = append(ranges, [2]int{lower, upper})

	return ranges
}

func byteSliceToBinaryString(byteSlice []byte) string {
	binaryString := ""
	for i := 0; i < len(byteSlice); i++ {
		binaryString = binaryString + reveres(fmt.Sprintf("%08b", byteSlice[i]))
	}
	return binaryString
}

func reveres(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func CreateEmptyReadResponse() ReadResponse {
	return ReadResponse{
		DiscreteInputs:        make(map[int]uint16),
		Coils:                 make(map[int]uint16),
		InputRegisters:        make(map[int]int16),
		HoldingRegisters:      make(map[int]int16),
		InputRegistersFloat32: make(map[int]float32),
		InputRegistersFloat64: make(map[int]float64),
	}
}

func CreateEmptyWriteResponse() WriteResponse {
	return WriteResponse{
		Coils:                  make(map[int]uint16),
		HoldingRegisters:       make(map[int]uint16),
		CoilsErrors:            make(map[int]error),
		HoldingRegistersErrors: make(map[int]error),
	}
}
