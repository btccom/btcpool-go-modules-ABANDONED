package merkle

import (
	"math"

	"merkle-tree-and-bitcoin/hash"
)

// MerkleTree is our implementation of a Merkle Tree. It comprises simply a
// sequence of Rows, representing the tiers in the tree, with the rows[0] being
// the leaf row. There are some notes in a comment at the end of this file
// about the thinking behind the choice of this data structure and storage.
type MerkleTree struct {
	rows []Row
}

// The Row type holds a sequence of hash values and represents one tier in the
// Merkle Tree.
type Row []hash.Byte32

// The MerklePath type models the Merkle Path of a given leaf in the Merkle
// Tree. It comprises essentially the sequence of hash values that must by
// cumulatively appended to the hash of a record before being re-hashed and
// moving on to repeat the process with the next value in the path. If this
// traversal algorithm produces the Merkle Root hash value at the end, the
// Merkle Path truth-test has been passed. The reason it is not just a sequence
// of hash values, is that the thing traversing the path needs also to know if
// the values found should be appended to the previous value or prepended
// before doing the re hash operation.
type MerklePath []MerklePathElement

// The MerklePathElement type is a trivial container that binds together a hash
// value and a flag to guide the Merkle Path traversal algorithm.
type MerklePathElement struct {
	Hash                    hash.Byte32
	UseFirstInConcatenation bool // me+other, not other+me
}

// NewMerkleTree is a factory that makes the Merkle Tree corresponding to a
// given set of leaf row hash values.
func NewMerkleTree(bottomRow Row) (tree MerkleTree) {
	// We install the bottom row and then work our way up making new rows,
	// derived from the one below - each approximating to half the length of
	// the one underneath. Stopping of course when we arrive at a row with just
	// one node - which is the Merkle Root by definition.
	tree.rows = append(tree.rows, bottomRow)
	rowBeneath := bottomRow
	for {
		if tree.isComplete() {
			break
		}
		rowAbove := makeRowAbove(rowBeneath)
		tree.rows = append(tree.rows, rowAbove)
		rowBeneath = rowAbove
	}
	return
}

// MerkleRoot is a simple API query function, that exists so that the row-based
// implementation details do not leak outside the object.
func (tree MerkleTree) MerkleRoot() hash.Byte32 {
	return tree.topRow()[0]
}

// MerklePathForLeaf is a an API method that builds the Merkle Path that
// corresponds to a given leaf in the tree. You specify which leaf by
// providing its record index. It works bottom up calculating which nodes in
// the tree are the siblings from which hashes should be collected.
func (tree MerkleTree) MerklePathForLeaf(leafIndex int) (
	merklePath MerklePath) {
	i := leafIndex
	// This iteration starts at the leaf row and consume all the row above
	// except the top one containing the Merkle Root.
	for _, row := range tree.rows[:len(tree.rows)-1] {
		sibling, useFirstInConcatenation := row.evaluateSibling(i)
		merklePathElement := MerklePathElement{
			Hash: row[sibling],
			UseFirstInConcatenation: useFirstInConcatenation}
		merklePath = append(merklePath, merklePathElement)
		// This division produces the index that should be used in the row
		// above to find a given node's parent. The truncating integer division
		// is deliberate and necessary to produce the same parent for two
		// adjacent nodes.
		i = i / 2
	}
	return
}

// CalculateMerkleRootFromMerklePath is a stand along function that more
// properly belongs out in client-world, where it will be used. Note that it is
// completely decoupled from any access to the Merkle Tree the path has been
// derived from, and does its work only from the dependency-injected arguments.
// It has been included in this module, so that a person studying the Merkle
// Tree logic can find it all in one place.
// This is the function that starts with the hash value of a record and
// consumes the Merkle Path one hash at a time by concatenating each hash
// encountered to the previously calculated value, before rehashing the
// concatenated bytes and repeating the process for the next hash in the
// path. Note that is seeks advice from the Merkle Path about in which order
// the elements should be concatenated at each level.
func CalculateMerkleRootFromMerklePath(
	leafHash hash.Byte32, merklePath MerklePath) hash.Byte32 {

	cumulativeHash := leafHash
	for _, merklePathElement := range merklePath {
		if merklePathElement.UseFirstInConcatenation {
			cumulativeHash = hash.JoinAndHash(
				merklePathElement.Hash, cumulativeHash)
		} else {
			cumulativeHash = hash.JoinAndHash(
				cumulativeHash, merklePathElement.Hash)
		}
	}
	return cumulativeHash
}

//---------------------------------------------------------------------------
// Private methods
//---------------------------------------------------------------------------

// isComplete() is a function that work out if a tree under construction is now
// complete by having reached a row with only one element in it. The function
// is trivial, and exists only to make the code calling it self-documenting.
func (tree MerkleTree) isComplete() bool {
	return len(tree.topRow()) == 1
}

// topRow is a function that returns the row at the end of the sequence of rows
// held by the tree. It is trivial, and exists only so that the fragile index
// arithmetic is restricted to just one place.
func (tree MerkleTree) topRow() Row {
	return tree.rows[len(tree.rows)-1]
}

/* evaluateSibling works out which neighbour of element X in a table row is
 * the sibling whose hash should be combined with that of X to form the parent
 * node hash. There is a general rule depending on if X is an even or odd
 * numbered element, plus a special case for X being at the right hand end of
 * an odd length row. In addition to identifying the sibling, this function
 * must capture the sequence in which the hash concatenations must be done, and
 * hence returns both the sibling index and a flag to signify the concatenation
 * order required. */
func (row Row) evaluateSibling(myIndex int) (
	siblingIndex int, useFirstInConcatenation bool) {

	// For all odd indices, the pair is leftNeighbour->me.
	// For most even indices, the pair is me->rightNeighbour
	// For the special case, the pair is me->me (by definition in Merkle Trees)

	if myIndex%2 == 1 {
		siblingIndex = myIndex - 1
		useFirstInConcatenation = true
	} else if (myIndex + 1) <= len(row)-1 {
		siblingIndex = myIndex + 1
		useFirstInConcatenation = false
	} else {
		siblingIndex = myIndex
		useFirstInConcatenation = true // moot
	}
	return
}

// makeRowAbove is a function that knows how to build a row in the tree in
// terms of the row that lives beneath it. It first works out how long the new
// row must be - which is half that of the one below - but rounded up. Then for
// each slot in this new row, it calculates the hash value to put in it, by
// locating its two child hashes, concatenating them, and hashing the
// concatenated bytes. It has to make special provision for the singularity
// that occurs when the row beneath has an odd length and consequently the
// parent at that end in the row above is calculated (by definition) by
// concatenating two copies of the only child available - i.e. the left child.
func makeRowAbove(below Row) Row {
	size := int(math.Ceil(float64(len(below)) / 2.0))
	row := make([]hash.Byte32, size)
	for i, _ := range row {
		leftChild := i * 2
		rightChild := leftChild + 1
		if rightChild <= len(below)-1 {
			row[i] = hash.JoinAndHash(below[leftChild], below[rightChild])
		} else {
			row[i] = hash.JoinAndHash(below[leftChild], below[leftChild])
		}
	}
	return row
}

/* Notes about the data structure used to model the tree.

There are several well known approaches to implementing tree data structures,
with various trade offs. In the real Bitcoin context, the domain is strongly
affected by the the concerns of scale, and it demands that we take care about:
the amount of memory we use, the expense of finely-grained separate memory
allocation events, and the computational speed efficiency of performing our
Merkle Tree operations.

It seems appropriate to respect these drivers in this implementation, despite
the fact that it is intended only to be educational. We of course also want to
make the code as simple as possible, to make it easy to understand and easy to
work on as well.

This solution is reasonably careful about memory consumption, by creating
storage only for hash values, rather than requiring storage of meta data such
as parent-child pointers or lookup tables. We instead use implicit navigation between children
and their parents by drawing on the well known binary tree idiom of doubling or
halving of array indices. We use Go's slices instead of raw arrays for each
row, mainly because this the idiomatic way to do arrays in Go, in most cases
and carries only a tiny space overhead. But we do construct these slices with a
calculated known size, in advance, both to avoid a reallocate-and-copy overhead
as we populate the row, and to defeat Go's automatic re-sizing during append,
which will inevitably create more headroom than is necessary sometimes. We
considered storing all the nodes in one single array, and retaining the
doubling/halving implicit navigation of parent/child relationships across all
the tiers - as is customary for heaps. But rejected it on two grounds. Firstly
it would need to almost twice the size strictly necessary when the number of
leaves was one greater than a power of 2. It would also make navigation only
possible upwards in the tree, because you would not be able to tell when
navigating downwards when you encountered a node that had no right hand child.
These ghost nodes could be represented by being populated by a sentinel value,
but there is none available in the set of legal hash values.

We are well behaved when it comes to avoiding finely grained memory allocations
by having a cost of only one allocation per row, per tree.

We avoid any computational complexity overhead by not maintaining any meta data
such as look up tables, and consequently building a tree with N leaves in this
implementation runs in roughly O(N). There is a bit to add which is
O(log_2[N]), but this becomes of minor significance for larger values of N.

Building a Merkle Path on demand for a given leaf on a tree with N leaves runs
in O(log_2[N]). This could of course be reduced to O(constant) if we were to
calculate and store the Merkle Paths for each node at construction time, but
this would violate our desire to conserve memory consumption.
*/
