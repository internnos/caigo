package caigo

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/dontpanicdao/caigo/types"
)

/*
Verifies the validity of the stark curve signature
given the message hash, and public key (x, y) coordinates
used to sign the message.

(ref: https://github.com/starkware-libs/cairo-lang/blob/master/src/starkware/crypto/starkware/crypto/signature/signature.py)
*/
func (sc StarkCurve) Verify(msgHash, r, s, pubX, pubY *types.Felt) bool {
	w := sc.InvModCurveSize(s)

	if s.Cmp(big.NewInt(0)) != 1 || s.Cmp(sc.N) != -1 {
		return false
	}
	if r.Cmp(big.NewInt(0)) != 1 || r.Cmp(sc.Max) != -1 {
		return false
	}
	if w.Cmp(big.NewInt(0)) != 1 || w.Cmp(sc.Max) != -1 {
		return false
	}
	if msgHash.Cmp(big.NewInt(0)) != 1 || msgHash.Cmp(sc.Max) != -1 {
		return false
	}
	if !sc.IsOnCurve(pubX, pubY) {
		return false
	}

	zGx, zGy, err := sc.MimicEcMultAir(msgHash, sc.EcGenX, sc.EcGenY, sc.MinusShiftPointX, sc.MinusShiftPointY)
	if err != nil {
		return false
	}

	rQx, rQy, err := sc.MimicEcMultAir(r, pubX, pubY, sc.Gx, sc.Gy)
	if err != nil {
		return false
	}
	inX, inY := sc.Add(zGx, zGy, rQx, rQy)
	wBx, wBy, err := sc.MimicEcMultAir(w, inX, inY, sc.Gx, sc.Gy)
	if err != nil {
		return false
	}

	outX, _ := sc.Add(wBx, wBy, sc.MinusShiftPointX, sc.MinusShiftPointY)
	if r.Cmp(outX) == 0 {
		return true
	} else {
		altY := new(big.Int).Neg(pubY)

		zGx, zGy, err = sc.MimicEcMultAir(msgHash, sc.EcGenX, sc.EcGenY, sc.MinusShiftPointX, sc.MinusShiftPointY)
		if err != nil {
			return false
		}

		rQx, rQy, err = sc.MimicEcMultAir(r, pubX, new(big.Int).Set(altY), sc.Gx, sc.Gy)
		if err != nil {
			return false
		}
		inX, inY = sc.Add(zGx, zGy, rQx, rQy)
		wBx, wBy, err = sc.MimicEcMultAir(w, inX, inY, sc.Gx, sc.Gy)
		if err != nil {
			return false
		}

		outX, _ = sc.Add(wBx, wBy, sc.MinusShiftPointX, sc.MinusShiftPointY)
		if r.Cmp(outX) == 0 {
			return true
		}
	}
	return false
}

/*
Signs the hash value of contents with the provided private key.
Secret is generated using a golang implementation of RFC 6979.
Implementation does not yet include "extra entropy" or "retry gen".

(ref: https://datatracker.ietf.org/doc/html/rfc6979)
*/
func (sc StarkCurve) Sign(msgHash, privKey *big.Int, seed ...*big.Int) (x, y *big.Int, err error) {
	if msgHash.Cmp(big.NewInt(0)) != 1 || msgHash.Cmp(sc.Max) != -1 {
		return x, y, fmt.Errorf("invalid bit length")
	}

	invalidK := true
	for invalidK {
		inSeed := big.NewInt(0)
		if len(seed) == 1 {
			inSeed = seed[0]
		}
		k := sc.GenerateSecret(new(big.Int).Set(msgHash), new(big.Int).Set(privKey), inSeed)

		r, _ := sc.EcMult(k, sc.EcGenX, sc.EcGenY)

		// DIFF: in classic ECDSA, we take int(x) % n.
		if r.Cmp(big.NewInt(0)) != 1 || r.Cmp(sc.Max) != -1 {
			// Bad value. This fails with negligible probability.
			continue
		}

		agg := new(big.Int).Mul(r, privKey)
		agg = agg.Add(agg, msgHash)

		if new(big.Int).Mod(agg, sc.N).Cmp(big.NewInt(0)) == 0 {
			// Bad value. This fails with negligible probability.
			continue
		}

		w := DivMod(k, agg, sc.N)
		if w.Cmp(big.NewInt(0)) != 1 || w.Cmp(sc.Max) != -1 {
			// Bad value. This fails with negligible probability.
			continue
		}

		s := sc.InvModCurveSize(w)
		return r, s, nil
	}

	return x, y, nil
}

/*
Hashes the contents of a given array using a golang Pedersen Hash implementation.

(ref: https://github.com/seanjameshan/starknet.js/blob/main/src/utils/ellipticCurve.ts)
*/
func (sc StarkCurve) HashElements(elems []*types.Felt) (hash *types.Felt, err error) {
	if len(elems) == 0 {
		elems = append(elems, types.StrToFelt("0"))
	}

	hash = types.BigToFelt(big.NewInt(0))
	for _, h := range elems {
		hash, err = sc.PedersenHash([]*types.Felt{hash, h})
		if err != nil {
			return hash, err
		}
	}
	return hash, err
}

/*
Provides the pedersen hash of given array of big integers.
NOTE: This function assumes the curve has been initialized with contant points

(ref: https://github.com/seanjameshan/starknet.js/blob/main/src/utils/ellipticCurve.ts)
*/
func (sc StarkCurve) PedersenHash(elems []*types.Felt) (hash *types.Felt, err error) {
	if len(sc.ConstantPoints) == 0 {
		return hash, fmt.Errorf("must initiate precomputed constant points")
	}

	// types.BigToFelt()
	// tempptx := new(big.Int).Set(sc.Gx)
	ptx := types.BigToFelt(new(big.Int).Set(sc.Gx))
	pty := types.BigToFelt(new(big.Int).Set(sc.Gy))
	for i, elem := range elems {
		x := elem

		if x.Cmp(big.NewInt(0)) != -1 && x.Cmp(sc.P) != -1 {
			return ptx, fmt.Errorf("invalid x: %v", x)
		}

		for j := 0; j < 252; j++ {
			idx := 2 + (i * 252) + j
			xin := types.BigToFelt(new(big.Int).Set(sc.ConstantPoints[idx][0]))
			yin := types.BigToFelt(new(big.Int).Set(sc.ConstantPoints[idx][1]))
			if xin.Cmp(ptx.Int) == 0 {
				return hash, fmt.Errorf("constant point duplication: %v %v", ptx, xin)
			}
			if x.Bit(0) == 1 {
				ptx, pty = sc.Add(ptx, pty, xin, yin)
			}
			xx := x.Int.Rsh(x.Int, 1)
			x = types.BigToFelt(xx)
		}
	}

	return ptx, nil
}

// implementation based on https://github.com/codahale/rfc6979/blob/master/rfc6979.go
func (sc StarkCurve) GenerateSecret(msgHash, privKey, seed *big.Int) (secret *big.Int) {
	alg := sha256.New
	holen := alg().Size()
	rolen := (sc.BitSize + 7) >> 3

	if msgHash.BitLen()%8 <= 4 && msgHash.BitLen() >= 248 {
		msgHash = msgHash.Mul(msgHash, big.NewInt(16))
	}

	by := append(int2octets(privKey, rolen), bits2octets(msgHash, sc.N, sc.BitSize, rolen)...)

	if seed.Cmp(big.NewInt(0)) == 1 {
		by = append(by, seed.Bytes()...)
	}

	v := bytes.Repeat([]byte{0x01}, holen)

	k := bytes.Repeat([]byte{0x00}, holen)

	k = mac(alg, k, append(append(v, 0x00), by...), k)

	v = mac(alg, k, v, v)

	k = mac(alg, k, append(append(v, 0x01), by...), k)

	v = mac(alg, k, v, v)

	for {
		var t []byte

		for len(t) < sc.BitSize/8 {
			v = mac(alg, k, v, v)
			t = append(t, v...)
		}

		secret = bits2int(new(big.Int).SetBytes(t), sc.BitSize)
		// TODO: implement seed here, final gating function
		if secret.Cmp(big.NewInt(0)) == 1 && secret.Cmp(sc.N) == -1 {
			return secret
		}
		k = mac(alg, k, append(v, 0x00), k)
		v = mac(alg, k, v, v)
	}
}
