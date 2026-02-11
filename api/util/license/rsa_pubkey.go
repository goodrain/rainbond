package license

import (
	"crypto/rsa"
)

// rsaPublicKeyPEM holds the embedded RSA public key.
// In production, replace this with the actual public key at build time.
var rsaPublicKeyPEM = []byte(`-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvGWiEisZHHAzuauvcelG
ThKRY28j7Q/XwzaDBAe4OF3rrDXfK01BIcBCAOa2eE8WzYNPCtz81+GkagNw5lIQ
nDcG8N4uF9rjO7ZQmQb2hy6wSbQozYOTnfE6aWQ68HGSpluhZbaDjkmFop/cfAhW
CobDwev/gE0nxRhOpx5TnMMWmc2VQJpncNFXGiGx7GRpxDRJ5gdbgtUDPFTssV1I
cKl82wXU1dURlsBQsBHvFbSOw3bwo/evygkRa4NwyjAucqKWM/BhvfdGFpZ2IKiY
cDLuRhKqWa5lK88hJJrcvOgl1Mb4vWN+gp3LlgZBP9N7uHKkzkVf939K3MOoXPaP
+qNsy6r7F01XGoF8+Ubq0dhHU0uv5G3etjbnxZsFilNRGZLPpaBxsqkkiQ3quS4F
i/NaZZwrTxkfGZCQmM7x24/gykA0A9ItZPvb25Emo7MUh3jpCjN5AP3bDNcU0YgP
6wHX7NMtNIKfLgRVe/f20dYQnP4hxSfHP7fQ87/jZebdF719pxLYbo8m2nn3OtuX
o1Inc1M4omeb8GCw/q/H+0rpa1eAB6IERJN+lwT8NZTWdAOQWMV7kLbbl8FnyYLu
I5qqCi5nusdNz+vTg4bkSmQ1YSH1/SmqAqz3p/2zfJWtlYC+DbjyB1LsBsg1ppno
/cnmeiqxYgk/OPjAdgaRfbcCAwEAAQ==
-----END PUBLIC KEY-----`)

// GetEmbeddedPublicKey parses and returns the embedded RSA public key.
func GetEmbeddedPublicKey() (*rsa.PublicKey, error) {
	return ParsePublicKey(rsaPublicKeyPEM)
}
