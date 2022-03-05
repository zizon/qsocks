package stream_test

import (
	"fmt"
	"testing"

	"github.com/zizon/qsocks/pkg/stream"
)

func TestFromStream(t *testing.T) {
	expetedErr := fmt.Errorf("done")
	source := make(chan int)

	s1 := stream.From(source)
	s2 := stream.Move(s1, func(v int) (string, error) {
		t.Logf("receive:%v", v)
		if v > 100 {
			t.Logf("trigger")

			return "", expetedErr
		}
		return fmt.Sprintf("value:%v", v), nil
	})
	stream.Drain(s2, nil)

	for i := 0; i < 200; i++ {
		t.Logf("send:%v", i)
		select {
		case <-s1.Done():
			break
		case source <- i:
		}
	}

	<-s2.Done()
	t.Logf("wait last")

	<-s1.Done()

	if s1.Err() != expetedErr {
		t.Errorf("expected:%v but:%v", expetedErr, s1.Err())
	} else {
		t.Logf("head done:%v", s1.Err())
	}
}

func TestOfStream(t *testing.T) {
	expetedErr := fmt.Errorf("done")
	i := 0
	s1 := stream.Of(func() (int, error) {
		i += 1
		return i, nil
	})
	s2 := stream.Move(s1, func(t int) (int, error) {
		if i > 100 {
			return 0, expetedErr
		}
		return i, nil
	})
	stream.Drain(s2, nil)

	<-s2.Done()
	t.Logf("wait last")

	<-s1.Done()

	if s1.Err() != expetedErr {
		t.Errorf("expected:%v but:%v", expetedErr, s1.Err())
	} else {
		t.Logf("head done:%v", s1.Err())
	}
}

func TestReplicate(t *testing.T) {
	expetedS1Err := fmt.Errorf("s1 done")
	expectedS2Err := fmt.Errorf("s2 done")
	i := 0
	s1 := stream.Of(func() (int, error) {
		i += 1
		return i, nil
	})

	// s1copy will ternimaite at most 100
	s2 := s1.Disentangle()
	stream.Drain(stream.Move(s2, func(v int) (int, error) {
		if v > 100 {
			return 0, expectedS2Err
		}

		return v, nil
	}), nil)

	stream.Drain(s1, func(v int) error {
		if v > 20000 {
			return expetedS1Err
		}

		return nil
	})

	<-s1.Done()
	<-s2.Done()

	if s1.Err() != expetedS1Err || s2.Err() != expectedS2Err {
		t.Errorf("expete s1(%v)=%v s2(%v)=%v", expetedS1Err, s1.Err(), expectedS2Err, s2.Err())
	}
}

func TestReduce(t *testing.T) {
	l := 1
	r := 2
	sum := stream.Reduce(stream.Once(1), stream.Once(2), func(l int, r int) (int, error) {
		return l + r, nil
	})

	stream.Drain(sum, func(v int) error {
		if v != l+r {
			return fmt.Errorf("not match")
		}
		return nil
	})

	<-sum.Done()
}
