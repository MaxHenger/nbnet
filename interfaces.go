package nbnet

type Encrypter interface {
	Encrypt(message []byte) (encrypted []byte, err error)
	Update() error
}

type Decrypter interface {
	Decrypt(message []byte) (decrypted []byte, err error)
}

type Signer interface {
	Sign(data []byte) ([]byte, error)
	Verify(encrypted, message []byte) (ok bool, err error)
}