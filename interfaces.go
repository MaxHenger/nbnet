package nbnet

type Encrypter interface {
	Encrypt(message []byte) (encrypted []byte, err error)
}

type Decrypter interface {
	Decrypt(message []byte) (decrypted []byte, err error)
}