package password

import "golang.org/x/crypto/bcrypt"

const cost = 13

// dummyHash is a pre-computed bcrypt hash used by VerifyConstantTime when
// the user has no stored hash (typically: user not found). Without this,
// login responses for non-existent usernames would return in <10 ms while
// existing users take ~250 ms — a textbook user-enumeration timing oracle.
//
// The plaintext "x" is irrelevant; we never expose VerifyConstantTime to
// authenticate against this hash. The hash exists purely to consume the
// same CPU as a real verification.
var dummyHash = mustHash("x")

func mustHash(plain string) []byte {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		panic("password: failed to generate dummy hash: " + err.Error())
	}
	return b
}

func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	return string(b), err
}

func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// VerifyConstantTime returns whether the plaintext matches the stored hash,
// while always spending one full bcrypt round even if hash is empty (user
// not found / no password set). Always prefer this in login paths.
func VerifyConstantTime(hash, plain string) bool {
	if hash == "" {
		// Spend a real bcrypt round so the caller's response time matches the
		// "user found, password wrong" path. Result is discarded — `plain`
		// can never legitimately match the random dummy.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
