package btcpay

import (
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"hash"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

// ecPrivateKey provides compatibility with the btcec package.
type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag1"`
}

// generatePEM generates a new PEM string.
func generatePEM() (string, error) {
	priv, err := btcec.NewPrivateKey(btcec.S256())
	if err != nil {
		return "", err
	}

	ecd := priv.PubKey().ToECDSA()
	oid := asn1.ObjectIdentifier{1, 3, 132, 0, 10}

	der, err := asn1.Marshal(ecPrivateKey{
		Version:       1,
		PrivateKey:    priv.D.Bytes(),
		NamedCurveOID: oid,
		PublicKey:     asn1.BitString{Bytes: elliptic.Marshal(btcec.S256(), ecd.X, ecd.Y)},
	})
	if err != nil {
		return "", err
	}

	v := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	return string(v), nil
}

// generateSIN generates a SIN string from the provided PEM string.
func generateSIN(pm string) (string, error) {
	pk, err := privateKey(pm)
	if err != nil {
		return "", err
	}

	pub := hex.EncodeToString(pk.PubKey().SerializeCompressed())
	hx, err := hexHash(sha256.New(), pub)
	if err != nil {
		return "", err
	}

	hx, err = hexHash(ripemd160.New(), hx)
	if err != nil {
		return "", err
	}

	sinHeader := "0F02" + hx

	hx, err = hexHash(sha256.New(), sinHeader)
	if err != nil {
		return "", err
	}

	hx, err = hexHash(sha256.New(), hx)
	if err != nil {
		return "", err
	}

	checksum := hx[0:8]
	hx = sinHeader + checksum

	bhx, err := hex.DecodeString(hx)
	if err != nil {
		return "", err
	}

	return base58.Encode(bhx), nil
}

// sign uses the private key in the PEM string to sign the provided value.
func sign(pm, v string) (string, error) {
	pk, err := privateKey(pm)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	if _, err = hash.Write([]byte(v)); err != nil {
		return "", err
	}

	sig, err := pk.Sign(hash.Sum(nil))
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sig.Serialize()), nil
}

// hexHash hashes the provided value with the specified hashing algorithm
// and returns its result in a hexadecimal format.
func hexHash(hash hash.Hash, v string) (string, error) {
	b, err := hex.DecodeString(v)
	if err != nil {
		return "", err
	}

	if _, err = hash.Write(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// privateKey extracts a private key from the provided PEM string.
func privateKey(pm string) (*btcec.PrivateKey, error) {
	b, _ := pem.Decode([]byte(pm))

	var ecpk ecPrivateKey

	if _, err := asn1.Unmarshal(b.Bytes, &ecpk); err != nil {
		return nil, err
	}

	priv, _ := btcec.PrivKeyFromBytes(btcec.S256(), ecpk.PrivateKey)

	return priv, nil
}

// publicKey extracts a compressed public key from the provided PEM string.
func publicKey(pm string) (string, error) {
	pk, err := privateKey(pm)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(pk.PubKey().SerializeCompressed()), nil
}
