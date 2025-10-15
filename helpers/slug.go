package helpers

import gonanoid "github.com/matoous/go-nanoid/v2"

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const length = 7

func New() (string, error) {
	return gonanoid.Generate(alphabet, length)
}