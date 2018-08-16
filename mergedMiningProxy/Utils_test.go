package main

import (
	"testing"
)

// 测试 BitsToTarget
func TestBitsToTarget(t *testing.T) {
	bits := "207fffff"
	expectTarget := "7fffff0000000000000000000000000000000000000000000000000000000000"
	target, err := BitsToTarget(bits)
	if err != nil {
		t.Error("failed: ", err)
	}
	if target != expectTarget {
		t.Errorf("bits: %s, expectTarget: %s, target: %s", bits, expectTarget, target)
	}
}

// 测试 TargetToBits
func TestTargetToBits(t *testing.T) {
	target := "7fffff0000000000000000000000000000000000000000000000000000000000"
	expectBits := "207fffff"

	bits, err := TargetToBits(target)
	if err != nil {
		t.Error("failed: ", err)
	}
	if bits != expectBits {
		t.Errorf("target: %s, expectBits: %s, bits: %s", target, expectBits, bits)
	}
}
