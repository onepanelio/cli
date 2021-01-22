package util

import "math/rand"

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandASCIIBytes generates n rando ASCII bytes
// adapted from source: https://github.com/kpbird/golang_random_string/blob/master/main.go
func RandASCIIBytes(n int) ([]byte, error) {
	output := make([]byte, n)
	// We will take n bytes, one byte for each character of output.
	randomness := make([]byte, n)
	// read all random
	_, err := rand.Read(randomness)
	if err != nil {
		return nil, err
	}

	l := len(letterBytes)
	// fill output
	for pos := range output {
		// get random item
		random := uint8(randomness[pos])
		// random % 64
		randomPos := random % uint8(l)
		// put into output
		output[pos] = letterBytes[randomPos]
	}

	return output, nil
}

// RandASCIIString returns a rnadom string up to n characters
func RandASCIIString(n int) (string, error) {
	res, err := RandASCIIBytes(n)
	if err != nil {
		return "", err
	}

	return string(res), nil
}
