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
