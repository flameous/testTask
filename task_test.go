package testTask

import (
	"testing"
	"fmt"
	"time"
	"errors"
	"strconv"
)

type TestCase struct {
	key    string
	getter func() (interface{}, error)
}

var (
	appleGetter TestCase = TestCase{
		key: "apple",
		getter: func() (interface{}, error) {
			time.Sleep(time.Second)
			return `apple getter`, nil
		},
	}

	pearGetter TestCase = TestCase{
		key: "pear",
		getter: func() (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return `pear getter`, nil
		},
	}

	dangerousGetter TestCase = TestCase{
		key: "orange",
		getter: func() (interface{}, error) {
			// p(A) = 30 %
			return 0, errors.New("Я всегда ломаюсь, это в порядке вещей!")
		},
	}
)

func TestGet(t *testing.T) {
	fmt.Println("TEST 1")

	tc := []TestCase{
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,

		pearGetter,
		pearGetter,
		pearGetter,

		dangerousGetter,
		dangerousGetter,
		dangerousGetter,
		dangerousGetter,
	}

	results := make(chan Result, len(tc))
	defer close(results)

	ts := time.Now()
	for _, v := range tc {
		go func(tc TestCase) {
			t2 := time.Now().Sub(ts).String()
			i, err := Get(tc.key, tc.getter)

			fmt.Printf("i: %-15v err: %-40v key: \"%s\". Time: %s\n",
				i, err, tc.key, t2)
			results <- Result{i, err}
		}(v)
	}

	for i := 0; i < len(tc); i++ {
		_ = <-results
	}
}

// первый геттер работает секунду, и за это время в очередь за результатом от такого же геттера
// набирается ещё N запросов, которым в канал отдаётся инфа
func TestGet2(t *testing.T) {
	fmt.Println("\n\n\nTEST 2")
	resetCache()

	tc := []TestCase{
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
	}

	results := make(chan Result, len(tc))
	defer close(results)

	ts := time.Now()

	for _, v := range tc {
		go func(tc TestCase) {
			i, err := Get(tc.key, tc.getter)
			t2 := time.Now().Sub(ts).String()
			fmt.Printf("i: %-15v err: %-40v key: \"%s\". Time: %s\n",
				i, err, tc.key, t2)
			results <- Result{i, err}
		}(v)
	}

	for i := 0; i < len(tc); i++ {
		_ = <-results
	}
}


// очистка кэша
func TestGet3(t *testing.T) {
	fmt.Println("\n\n\nTEST 3")
	resetCache()

	tc := []TestCase{
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
		appleGetter,
	}

	results := make(chan Result, len(tc))
	defer close(results)

	ts := time.Now()

	for k, v := range tc {

		if k == 1 {
			time.Sleep(3 * time.Second)
		} else {
			time.Sleep( 500 * time.Millisecond)
		}

		go func(tc TestCase) {
			i, err := Get(tc.key, tc.getter)
			t2 := time.Now().Sub(ts).String()
			fmt.Printf("i: %-15v err: %-40v key: \"%s\". Time: %s\n",
				i, err, tc.key, t2)
			results <- Result{i, err}
		}(v)
	}

	for i := 0; i < len(tc); i++ {
		_ = <-results
	}

}


// 10k уникальных геттеров, асинхронно
func TestGet4(t *testing.T) {
	fmt.Println("\n\n\nTEST 4")
	resetCache()

	n := 10000

	results := make(chan Result, n)
	defer close(results)

	for i := 0; i < n; i++ {
		var v TestCase
		switch i % 3 {
		case 0:
			v = appleGetter
		case 1:
			v = pearGetter
		default:
			v = dangerousGetter
		}


		go func(tc TestCase) {
			thisIsTotallyUniqueKey := tc.key + strconv.Itoa(n)
			i, err := Get(thisIsTotallyUniqueKey, tc.getter)
			results <- Result{i, err}
		}(v)
	}

	for i := 0; i < n; i++ {
		_ = <-results
	}

}