package hash

import (
	"encoding/hex"
	"testing"
)

// These constants are reference SHA-256 hash values, sourced from
// http://www.xorbin.com/tools/sha256-hash-calculator
const hashOfLetterA = "559aead08264d5795d3909718cdd05abd49572e84fe55590eef31a88a08fdffd"

// TestHexFormatter ensures that the Hex method on the Hash type produces the
// expected value for a reference test case.
func TestHexFormatter(t *testing.T) {
	hash := Hash([]byte("A"))
	produced := hash.Hex()
	expected := hashOfLetterA
	if produced != expected {
		t.Errorf(
			"\nResponse:\n%s\ndiffers from expected:\n%s", produced, expected)
	}
}

// TestHashWrapperFunctionWithKnownTestVector ensures that the wrapping of Go's
// native SHA256 hash function by the hash.Hash function, produces correct
// results on an externally sourced test vector.
func TestHashWrapperFunctionWithKnownTestVector(t *testing.T) {
	// Source of test vectors:
	// http://www.di-mgt.com.au/sha_testvectors.html#FIPS-180
	testInput := []byte("abc") // Yes. That is official test vector.
	expectedOutput :=
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

	res := Hash(testInput).Hex()
	if res != expectedOutput {
		t.Errorf(
			"\nResponse:\n%s\ndiffers from expected:\n%s", res, expectedOutput)
	}
}

// TestJoinAndHash checks that hash.JoinAndHash properly concatenates and
// re-hashes two input hash values, by comparing the result on test vectors
// with the result produced by doing it a different way - namely converting
// externally sourced hex string representations of the two hashes to be
// joined into their byte stream representations, and manually concatenating
// these byte streams before rehashing them.
func TestJoinAndHash(t *testing.T) {
	// We will use the characters 'A' and 'B' as our reference inputs.

	// First use the hash.JoinAndHash function.
	hashA := Hash([]byte("A"))
	hashB := Hash([]byte("B"))
	joinedAndHashed := JoinAndHash(hashA, hashB).Hex()

	// Now replicate the same thing using hex string decoding and
	// concatenation.

	// These values are reference inputs from:
	// http://www.xorbin.com/tools/sha256-hash-calculator

	hexDerivedA, err := hex.DecodeString(
		"559aead08264d5795d3909718cdd05abd49572e84fe55590eef31a88a08fdffd")
	if err != nil {
		t.Errorf("hex.DecodeString failed with: %s", err.Error())
	}
	hexDerivedB, err := hex.DecodeString(
		"df7e70e5021544f4834bbee64a9e3789febc4be81470df629cad6ddb03320a5c")
	if err != nil {
		t.Errorf("hex.DecodeString failed with: %s", err.Error())
	}
	concatenatedBytes := []byte{}
	concatenatedBytes = append(concatenatedBytes, hexDerivedA...)
	concatenatedBytes = append(concatenatedBytes, hexDerivedB...)

	hashFromManualConcatenation :=
		Hash(concatenatedBytes).Hex()

	if joinedAndHashed != hashFromManualConcatenation {
		t.Errorf(
			"\njoinedAndHashed:\n%s\ndiffers from "+
				"hashFromManualConcatenation:\n%s",
			joinedAndHashed,
			hashFromManualConcatenation)
	}
}
