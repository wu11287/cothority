package initial

import (
	"fmt"
	"testing"
)

func BenchmarkMain(b *testing.B) {
	for i := 0; i < b.N; i++{
		var end float64
		for j := 0; j < 3; j++ {
			end += count()
		}
		fmt.Println(end/3.0)
	}
}