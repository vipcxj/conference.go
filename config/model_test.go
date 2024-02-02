package config_test

import (
	"fmt"
	"sync"
	"testing"
)

type Test struct {
	id string
}

func NewTest(id string) *Test {
	return &Test{
		id: id,
	}
}

func (t *Test) Close() {
	fmt.Printf("close %s\n", t.id)
}

type TestBox struct {
	test *Test
}

func (t *TestBox) Close() {
	t.test.Close()
}

func TestDefer(t *testing.T) {
	test := &TestBox{
		test: NewTest("1"),
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func ()  {
		defer test.Close()
		test.test = NewTest("2")	
		wg.Done()
	}()
	wg.Wait()
}

type Test3 struct {
	A string
}

func (t *Test3) String() string {
	return fmt.Sprintf("{A: %s}", t.A)
}

type Test2 struct {
	A int
	B Test3
	C *Test3
}

func (t *Test2) String() string {
	return fmt.Sprintf("{A: %d, B: %v, C: %v}", t.A, t.B, t.C)
}

func TestCopy(t *testing.T) {
	t3 := Test3{
		A: "a3",
	}
	test := Test2{
		A: 1,
		B: t3,
		C: &t3,
	}
	testCpy := test
	test.A = 2
	test.B.A = "a1"
	test.C.A = "a2"
	fmt.Printf("test: %v\n", test)
	fmt.Printf("testCpy: %v\n", testCpy)
}