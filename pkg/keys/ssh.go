package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"golang.org/x/crypto/ssh"
)

// KeyBits is the default key size to generate, in bits.
//
// IMPORTANT: For GCP it has to be 3072 bit key. When we tried to use 2048
// or 4096, it was not adding it to the authorized_keys file on cluster
// nodes, even if the key was present in a gcp console (in node's
// metadata).
const KeyBits = 3072

// GeneratePrivateKey generates a new RSA private kwy, writing it to
// the file named by path.
func GeneratePrivateKey(path string) error {
	priv, err := rsa.GenerateKey(rand.Reader, KeyBits)
	if err != nil {
		return err
	}

	b := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}

	defer f.Close()
	return pem.Encode(f, &b)
}

// NewPublicKey attempts to read RSA private key from path, generating a
// new key at that path if it doesn't exist. It returns the corresponding
// SSH public key.
func NewPublicKey(path string) (ssh.PublicKey, error) {
	k, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		if err := GeneratePrivateKey(path); err != nil {
			return nil, err
		}

		k, err = ioutil.ReadFile(path)
	}

	if err != nil {
		return nil, err
	}

	b, _ := pem.Decode(k)
	if b == nil {
		return nil, fmt.Errorf("no PEM data in %q", path)
	}
	if b.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("wrong key type %q", b.Type)
	}

	priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
	if err != nil {
		return nil, err
	}

	return ssh.NewPublicKey(&priv.PublicKey)
}
