package protocol

import (
	"math/rand"
	"time"
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	adj := adjectives[r.Intn(len(adjectives))]
	fruit := fruits[r.Intn(len(fruits))]
	return adj + " " + fruit
}
