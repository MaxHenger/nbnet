package nbnet

type EncryptionState interface {
	IV() []byte
	Key() []byte
	Update(encrypted []byte) error
}

type Encrypter interface {
	Encrypt(state EncryptionState, message []byte) (encrypted []byte, err error)
}

type Decrypter interface {
	Decrypt(state EncryptionState, message []byte) (decrypted []byte, err error)
}

type Signer interface {
	Sign(state EncryptionState, data []byte) ([]byte, error)
	Verify(state EncryptionState, encrypted, message []byte) (ok bool, err error)
}