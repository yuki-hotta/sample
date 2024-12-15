package main

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func Test_Main(t *testing.T) {
	defer goleak.VerifyNone(t)

	main()
}
