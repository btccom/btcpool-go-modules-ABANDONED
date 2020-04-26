package main

import (
	"fmt"
	"strconv"
	"strings"
)

func parseHashrate(hashrate string) (float64, error) {
	suffix := map[string]float64{
		"K": 1e3,
		"M": 1e6,
		"G": 1e9,
		"T": 1e12,
		"P": 1e15,
		"E": 1e18,
		"Z": 1e21,
		"Y": 1e24,
	}

	if len(hashrate) == 0 {
		return 0, nil
	}

	if base, ok := suffix[strings.ToUpper(hashrate[len(hashrate)-1:])]; ok {
		num, err := strconv.ParseFloat(hashrate[0:len(hashrate)-1], 64)
		if err != nil {
			return 0, err
		}
		return num * base, nil
	}

	num, err := strconv.ParseFloat(hashrate, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func formatHashrate(hashrate float64) string {
	if hashrate >= 1e24 {
		return fmt.Sprintf("%0.2fY", hashrate/1e24)
	}
	if hashrate >= 1e21 {
		return fmt.Sprintf("%0.2fZ", hashrate/1e21)
	}
	if hashrate >= 1e18 {
		return fmt.Sprintf("%0.2fE", hashrate/1e18)
	}
	if hashrate >= 1e15 {
		return fmt.Sprintf("%0.2fP", hashrate/1e15)
	}
	if hashrate >= 1e12 {
		return fmt.Sprintf("%0.2fT", hashrate/1e12)
	}
	if hashrate >= 1e9 {
		return fmt.Sprintf("%0.2fG", hashrate/1e9)
	}
	if hashrate >= 1e6 {
		return fmt.Sprintf("%0.2fM", hashrate/1e6)
	}
	if hashrate >= 1e3 {
		return fmt.Sprintf("%0.2fk", hashrate/1e3)
	}
	return fmt.Sprintf("%0.2f", hashrate)
}

func getHashrateBase(chain string) float64 {
	baseMap := map[string]float64{
		"btc":  4294967296,
		"bcc":  4294967296,
		"bch":  4294967296,
		"bsv":  4294967296,
		"ubtc": 4294967296,
		"eth":  1,
		"etc":  1,
	}
	if base, ok := baseMap[chain]; ok {
		return base
	}
	return 0
}
