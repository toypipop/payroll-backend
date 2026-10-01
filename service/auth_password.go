package service

import "golang.org/x/crypto/bcrypt"

// Generate once, not per failed login. Use the same cost as employee password
// creation; existing legacy hashes may use a different cost. This is public
// dummy data, not a credential, and a match never grants access.
var dummyLoginHash = func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("missing-account-dummy-password"), bcryptCost)
	if err != nil {
		panic("unable to initialize dummy login hash")
	}
	return hash
}()
