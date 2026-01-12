package application

import "math/rand"

type Random struct {
	Rng *rand.Rand
}

func NewRandom() *Random {
	return &Random{Rng: rand.New(rand.NewSource(33))}
}

// GetRandomId in my realization ids start from 0
func (r *Random) GetRandomId(idsCount int32) int32 {
	return r.Rng.Int31() % idsCount

}
