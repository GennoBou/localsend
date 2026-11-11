package protocol

import (
	cryptoRand "crypto/rand"
	"math/big"
)

var adjectives = []string{
	"Fast", "Slow", "Nice", "Warm", "Cold", "Beautiful", "Smart", "Cool", "Sweet", "Sour",
	"Fresh", "Cute", "Strong", "Gentle", "Happy", "Sad", "Angry", "Calm", "Wild", "Crazy",
}

var fruits = []string{
	"Apple", "Banana", "Cherry", "Date", "Elderberry", "Fig", "Grape", "Honeydew", "Kiwi",
	"Lemon", "Mango", "Nectarine", "Orange", "Peach", "Pear", "Plum", "Quince", "Raspberry",
	"Strawberry", "Tomato",
}

// GenerateRandomAlias generates a random LocalSend-style alias combining an adjective and a fruit.
func GenerateRandomAlias() string {
	adjIdx, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return adjectives[0] + " " + fruits[0]
	}
	fruitIdx, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(int64(len(fruits))))
	if err != nil {
		return adjectives[0] + " " + fruits[0]
	}
	return adjectives[adjIdx.Int64()] + " " + fruits[fruitIdx.Int64()]
}
