package kademlia

import (
	"github.com/FavorLabs/favorX/pkg/boson"
	"math"
	"math/bits"
)

// generateCommonBinPrefixes generates the common bin prefixes
// used by the bin balancer.
func generateCommonBinPrefixes(base boson.Address, suffixLength int) [][]boson.Address {
	bitCombinationsCount := int(math.Pow(2, float64(suffixLength)))
	bitSuffixes := make([]uint8, bitCombinationsCount)

	for i := 0; i < bitCombinationsCount; i++ {
		bitSuffixes[i] = uint8(i)
	}

	addr := boson.MustParseHexAddress(base.String())
	addrBytes := addr.Bytes()
	_ = addrBytes

	binPrefixes := make([][]boson.Address, int(boson.MaxBins))

	// copy base address
	for i := range binPrefixes {
		binPrefixes[i] = make([]boson.Address, bitCombinationsCount)
	}

	for i := range binPrefixes {
		for j := range binPrefixes[i] {
			pseudoAddrBytes := make([]byte, len(base.Bytes()))
			copy(pseudoAddrBytes, base.Bytes())
			binPrefixes[i][j] = boson.NewAddress(pseudoAddrBytes)
		}
	}

	for i := range binPrefixes {
		for j := range binPrefixes[i] {
			pseudoAddrBytes := binPrefixes[i][j].Bytes()

			if len(pseudoAddrBytes) < 1 {
				continue
			}

			// flip first bit for bin
			indexByte, posBit := i/8, i%8
			if hasBit(bits.Reverse8(pseudoAddrBytes[indexByte]), uint8(posBit)) {
				pseudoAddrBytes[indexByte] = bits.Reverse8(clearBit(bits.Reverse8(pseudoAddrBytes[indexByte]), uint8(posBit)))
			} else {
				pseudoAddrBytes[indexByte] = bits.Reverse8(setBit(bits.Reverse8(pseudoAddrBytes[indexByte]), uint8(posBit)))
			}

			// set pseudo suffix
			bitSuffixPos := suffixLength - 1
			for l := i + 1; l < i+suffixLength+1; l++ {
				index, pos := l/8, l%8

				if hasBit(bitSuffixes[j], uint8(bitSuffixPos)) {
					pseudoAddrBytes[index] = bits.Reverse8(setBit(bits.Reverse8(pseudoAddrBytes[index]), uint8(pos)))
				} else {
					pseudoAddrBytes[index] = bits.Reverse8(clearBit(bits.Reverse8(pseudoAddrBytes[index]), uint8(pos)))
				}

				bitSuffixPos--
			}

			// clear rest of the bits
			for l := i + suffixLength + 1; l < len(pseudoAddrBytes)*8; l++ {
				index, pos := l/8, l%8
				pseudoAddrBytes[index] = bits.Reverse8(clearBit(bits.Reverse8(pseudoAddrBytes[index]), uint8(pos)))
			}
		}
	}

	return binPrefixes
}

// Clears the bit at pos in n.
func clearBit(n, pos uint8) uint8 {
	mask := ^(uint8(1) << pos)
	return n & mask
}

// Sets the bit at pos in the integer n.
func setBit(n, pos uint8) uint8 {
	return n | 1<<pos
}

func hasBit(n, pos uint8) bool {
	return n&(1<<pos) > 0
}
