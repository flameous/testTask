package testTask

import (
	"testing"
	"fmt"
)

type TestCase struct {
	key    string
	getter func() (interface{}, error)
}

var tc1 []TestCase

func TestGet(t *testing.T) {
	tc1 = []TestCase{
		TestCase{
			key: "apple",
			getter: func() (interface{}, error) {
				return 12, nil
			},
		},
		TestCase{
			key: "apple",
			getter: func() (interface{}, error) {
				return 4, nil
			},
		},
		TestCase{
			key: "pear",
			getter: func() (interface{}, error) {
				return 7, nil
			},
		},
	}

	results := make(chan Result, len(tc1))
	defer close(results)

	for _, v := range tc1 {
		go func() {
			i, err := Get(v.key, v.getter)
			results <- Result{i, err}
		}()
	}

	for i := 0; i < len(tc1); i++ {
		r := <- results
		fmt.Println(r)
	}

}
