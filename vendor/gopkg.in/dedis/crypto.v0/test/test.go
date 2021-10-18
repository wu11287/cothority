package test

import (
	"bytes"
	"crypto/cipher"

	"github.com/dedis/crypto/abstract"
	"gopkg.in/dedis/crypto.v0/random"
)

func testEmbed(g abstract.Group, rand cipher.Stream, points *[]abstract.Point,
	s string) {
	//println("embedding: ",s)
	b := []byte(s)

	p, rem := g.Point().Pick(b, rand)
	//println("embedded, remainder",len(rem),"/",len(b),":",string(rem))
	x, err := p.Data()
	if err != nil {
		panic("Point extraction failed: " + err.Error())
	}
	//println("extracted data: ",string(x))

	if !bytes.Equal(append(x, rem...), b) {
		panic("Point embedding corrupted the data")
	}

	*points = append(*points, p)
}

func testPointSet(g abstract.Group, rand cipher.Stream) {
	N := 1000
	null := g.Point().Null()
	for i := 0; i < N; i++ {
		P1, _ := g.Point().Pick(nil, rand)
		P2 := g.Point()
		P2.Set(P1)
		if !P1.Equal(P2) {
			panic("Set() set to a different point.")
		}
		if !P1.Equal(null) {
			P1.Add(P1, P1)
			if P1.Equal(P2) {
				panic("Modifying P1 shouldn't modify P2")
			}
		}
	}
}

func testPointClone(g abstract.Group, rand cipher.Stream) {
	N := 1000
	null := g.Point().Null()
	for i := 0; i < N; i++ {
		P1, _ := g.Point().Pick(nil, rand)
		P2 := P1.Clone()
		if !P1.Equal(P2) {
			panic("Clone didn't create a point with same " +
				"coordinates as the original point.")
		}
		if !P1.Equal(null) {
			P1.Add(P1, P1)
			if P1.Equal(P2) {
				panic("Modifying P1 shouldn't modify P2")
			}
		}
	}
}

func testScalarSet(g abstract.Group, rand cipher.Stream) {
	N := 1000
	one := g.Scalar().One()
	for i := 0; i < N; i++ {
		s1 := g.Scalar().Pick(rand)
		s2 := g.Scalar().Set(s1)
		if !s1.Equal(s2) {
			panic("Clone didn't create a scalar s2 with same value as s1's.")
		}
		if !s1.Equal(one) {
			s1.Mul(s1, s1)
			if s1.Equal(s2) {
				panic("Modifying s1 shouldn't modify s2")
			}
		}
	}
}

func testScalarClone(g abstract.Group, rand cipher.Stream) {
	N := 1000
	one := g.Scalar().One()
	for i := 0; i < N; i++ {
		s1 := g.Scalar().Pick(rand)
		s2 := s1.Clone()
		if !s1.Equal(s2) {
			panic("Clone didn't create a scalar s2 with same value as s1's.")
		}
		if !s1.Equal(one) {
			s1.Mul(s1, s1)
			if s1.Equal(s2) {
				panic("Modifying s1 shouldn't modify s2")
			}
		}
	}
}

// Apply a generic set of validation tests to a cryptographic Group,
// using a given source of [pseudo-]randomness.
//
// Returns a log of the pseudorandom Points produced in the test,
// for comparison across alternative implementations
// that are supposed to be equivalent.
//
func testGroup(g abstract.Group, rand cipher.Stream) []abstract.Point {
	//	fmt.Printf("\nTesting group '%s': %d-byte Point, %d-byte Scalar\n",
	//			g.String(), g.PointLen(), g.ScalarLen())

	points := make([]abstract.Point, 0)
	ptmp := g.Point()
	stmp := g.Scalar()
	pzero := g.Point().Null()
	szero := g.Scalar().Zero()
	sone := g.Scalar().One()

	// Do a simple Diffie-Hellman test
	s1 := g.Scalar().Pick(rand)
	s2 := g.Scalar().Pick(rand)
	if s1.Equal(s2) {
		panic("uh-oh, not getting unique secrets!")
	}

	gen := g.Point().Base()
	points = append(points, gen)

	// Verify additive and multiplicative identities of the generator.
	ptmp.Mul(nil, stmp.SetInt64(-1)).Add(ptmp, gen)
	if !ptmp.Equal(pzero) {
		panic("oops, generator additive identity doesn't work")
	}
	if g.PrimeOrder() { // secret.Inv works only in prime-order groups
		ptmp.Mul(nil, stmp.SetInt64(2)).Mul(ptmp, stmp.Inv(stmp))
		if !ptmp.Equal(gen) {
			panic("oops, generator multiplicative identity doesn't work")
		}
	}

	p1 := g.Point().Mul(gen, s1)
	p2 := g.Point().Mul(gen, s2)
	if p1.Equal(p2) {
		panic("uh-oh, encryption isn't producing unique points!")
	}
	points = append(points, p1)

	dh1 := g.Point().Mul(p1, s2)
	dh2 := g.Point().Mul(p2, s1)
	if !dh1.Equal(dh2) {
		panic("Diffie-Hellman didn't work")
	}
	points = append(points, dh1)
	//println("shared secret = ",dh1.String())

	// Test secret inverse to get from dh1 back to p1
	if g.PrimeOrder() {
		ptmp.Mul(dh1, g.Scalar().Inv(s2))
		if !ptmp.Equal(p1) {
			panic("Scalar inverse didn't work")
		}
	}

	// Zero and One identity secrets
	//println("dh1^0 = ",ptmp.Mul(dh1, szero).String())
	if !ptmp.Mul(dh1, szero).Equal(pzero) {
		panic("Encryption with secret=0 didn't work")
	}
	if !ptmp.Mul(dh1, sone).Equal(dh1) {
		panic("Encryption with secret=1 didn't work")
	}

	// Additive homomorphic identities
	ptmp.Add(p1, p2)
	stmp.Add(s1, s2)
	pt2 := g.Point().Mul(gen, stmp)
	if !pt2.Equal(ptmp) {
		panic("Additive homomorphism doesn't work")
	}
	ptmp.Sub(p1, p2)
	stmp.Sub(s1, s2)
	pt2.Mul(gen, stmp)
	if !pt2.Equal(ptmp) {
		panic("Additive homomorphism doesn't work")
	}
	st2 := g.Scalar().Neg(s2)
	st2.Add(s1, st2)
	if !stmp.Equal(st2) {
		panic("Scalar.Neg doesn't work")
	}
	pt2.Neg(p2).Add(pt2, p1)
	if !pt2.Equal(ptmp) {
		panic("Point.Neg doesn't work")
	}

	// Multiplicative homomorphic identities
	stmp.Mul(s1, s2)
	if !ptmp.Mul(gen, stmp).Equal(dh1) {
		panic("Multiplicative homomorphism doesn't work")
	}
	if g.PrimeOrder() {
		st2.Inv(s2)
		st2.Mul(st2, stmp)
		if !st2.Equal(s1) {
			panic("Scalar division doesn't work")
		}
		st2.Div(stmp, s2)
		if !st2.Equal(s1) {
			panic("Scalar division doesn't work")
		}
	}

	// Test randomly picked points
	last := gen
	for i := 0; i < 5; i++ {
		rgen, _ := g.Point().Pick(nil, rand)
		if rgen.Equal(last) {
			panic("Pick() not producing unique points")
		}
		last = rgen

		ptmp.Mul(rgen, stmp.SetInt64(-1)).Add(ptmp, rgen)
		if !ptmp.Equal(pzero) {
			panic("random generator fails additive identity")
		}
		if g.PrimeOrder() {
			ptmp.Mul(rgen, stmp.SetInt64(2)).Mul(ptmp, stmp.Inv(stmp))
			if !ptmp.Equal(rgen) {
				panic("random generator fails multiplicative identity")
			}
		}
		points = append(points, rgen)
	}

	// Test embedding data
	testEmbed(g, rand, &points, "Hi!")
	testEmbed(g, rand, &points, "The quick brown fox jumps over the lazy dog")

	// Test verifiable secret sharing
	// XXX re-enable when we move this into 'test' sub-package
	//testSharing(g)

	// Test encoding and decoding
	buf := new(bytes.Buffer)
	for i := 0; i < 5; i++ {
		buf.Reset()
		s := g.Scalar().Pick(rand)
		if _, err := s.MarshalTo(buf); err != nil {
			panic("encoding of secret fails: " + err.Error())
		}
		if _, err := stmp.UnmarshalFrom(buf); err != nil {
			panic("decoding of secret fails: " + err.Error())
		}
		if !stmp.Equal(s) {
			panic("decoding produces different secret than encoded")
		}

		buf.Reset()
		p, _ := g.Point().Pick(nil, rand)
		if _, err := p.MarshalTo(buf); err != nil {
			panic("encoding of point fails: " + err.Error())
		}
		if _, err := ptmp.UnmarshalFrom(buf); err != nil {
			panic("decoding of point fails: " + err.Error())
		}
		if !ptmp.Equal(p) {
			panic("decoding produces different point than encoded")
		}
	}

	// Test that we can marshal/ unmarshal null point
	pzero = g.Point().Null()
	b, _ := pzero.MarshalBinary()
	repzero := g.Point()
	err := repzero.UnmarshalBinary(b)
	if err != nil {
		panic(err)
	}

	testPointSet(g, rand)
	testPointClone(g, rand)
	testScalarSet(g, rand)
	testScalarClone(g, rand)

	return points
}

// Apply a generic set of validation tests to a cryptographic Group.
func TestGroup(g abstract.Group) {
	testGroup(g, random.Stream)
}

// Test two group implementations that are supposed to be equivalent,
// and compare their results.
func TestCompareGroups(suite abstract.Suite, g1, g2 abstract.Group) {

	// Produce test results from the same pseudorandom seed
	r1 := testGroup(g1, suite.Cipher(abstract.NoKey))
	r2 := testGroup(g2, suite.Cipher(abstract.NoKey))

	// Compare resulting Points
	for i := range r1 {
		b1, _ := r1[i].MarshalBinary()
		b2, _ := r2[i].MarshalBinary()
		if !bytes.Equal(b1, b2) {
			println("result-pair", i,
				"\n1:", r1[i].String(),
				"\n2:", r2[i].String())
			panic("unequal results")
		}
	}
}

// Apply a standard set of validation tests to a ciphersuite.
func TestSuite(suite abstract.Suite) {

	// Try hashing something
	h := suite.Hash()
	l := h.Size()
	//println("HashLen: ",l)
	h.Write([]byte("abc"))
	hb := h.Sum(nil)
	//println("Hash:")
	//println(hex.Dump(hb))
	if h.Size() != l || len(hb) != l {
		panic("inconsistent hash output length")
	}

	// Generate some pseudorandom bits
	s := suite.Cipher(hb)
	sb := make([]byte, 128)
	s.XORKeyStream(sb, sb)
	//println("Stream:")
	//println(hex.Dump(sb))

	// Test if it generates two fresh keys with nil cipher
	s1 := suite.NewKey(nil)
	s2 := suite.NewKey(nil)
	if s1.Equal(s2) {
		panic("NewKey returns twice the same key given nil")
	}

	// Test if it creates the same with the same seed
	st1 := suite.Cipher(hb)
	st2 := suite.Cipher(hb)
	s3 := suite.NewKey(st1)
	s4 := suite.NewKey(st2)
	if !s3.Equal(s4) {
		panic("NewKey returns two different keys given same stream")
	}

	// Test if it creates two different with random stream
	stream := random.Stream
	s5 := suite.NewKey(stream)
	s6 := suite.NewKey(stream)
	if s5.Equal(s6) {
		panic("NewKey returns same key given random stream")
	}

	// Test the public-key group arithmetic
	TestGroup(suite)
}
