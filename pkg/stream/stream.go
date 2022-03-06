package stream

import (
	"context"
	"fmt"
	"sync"
)

type State[T any] interface {
	context.Context

	Disentangle() State[T]

	unsafe() *state[T]
}

func Move[T any, U any](up State[T], apply func(T) (U, error)) State[U] {
	return move(up.unsafe(), apply)
}

func From[T any](source <-chan T) State[T] {
	return from(source)
}

func Flatten[T any, U any](from State[T], apply func(T) (State[U], error), order bool) State[U] {
	return flatten(from.unsafe(), func(v T) (*state[U], error) {
		u, err := apply(v)
		if err != nil {
			return nil, err
		}
		return u.unsafe(), nil
	}, order)
}

func Drain[T any](up State[T], apply func(T) error) {
	drain(up.unsafe(), apply)
}

func Of[T any](gen func() (T, error)) State[T] {
	return of(gen)
}

func Once[T any](v T) State[T] {
	return once(v)
}

func Reduce[L any, R any, O any](l State[L], r State[R], apply func(L, R) (O, error)) State[O] {
	return reduce(l.unsafe(), r.unsafe(), apply)
}

type state[T any] struct {
	sync.Locker

	error
	stream chan T

	context.Context
	context.CancelFunc
}

func (s *state[T]) Disentangle() State[T] {
	return from(s.stream)
}

func (s *state[T]) Err() error {
	s.Lock()
	defer s.Unlock()
	return s.error
}

func (s *state[T]) unsafe() *state[T] {
	return s
}

func (s *state[T]) abort(err error) {
	s.Lock()
	defer s.Unlock()
	if err != nil {
		s.error = err
	}

	s.CancelFunc()
}

func once[T any](value T) *state[T] {
	ch := make(chan T, 1)
	ch <- value
	close(ch)
	return from(ch)
}

func from[T any](source <-chan T) *state[T] {
	ctx, cancle := context.WithCancel(context.TODO())

	s := &state[T]{
		Locker: &sync.Mutex{},

		stream: make(chan T),

		Context:    ctx,
		CancelFunc: cancle,
	}
	go func() {
		defer close(s.stream)
		for {
			select {
			case v, more := <-source:
				if !more {
					s.abort(s.Err())
					return
				}

				// listen for cancelation
				select {
				case s.stream <- v:
					continue
				case <-s.Done():
					s.abort(s.Err())
					return
				}
			case <-s.Done():
				return
			}
		}
	}()

	return s
}

func drain[T any](up *state[T], apply func(T) error) {
	if apply == nil {
		apply = func(T) error {
			return nil
		}
	}

	go func() {
		for {
			select {
			case v, more := <-up.stream:
				if !more {
					// steram clsoed is not necessary imply
					// context is canceled
					up.abort(up.Err())
					return
				}
				if err := apply(v); err != nil {
					up.abort(err)
					return
				}
			case <-up.Done():
				return
			}
		}
	}()
}

func of[T any](gen func() (T, error)) *state[T] {
	ctx, cancle := context.WithCancel(context.TODO())

	s := &state[T]{
		Locker: &sync.Mutex{},

		stream: make(chan T),

		Context:    ctx,
		CancelFunc: cancle,
	}

	go func() {
		for {
			v, err := gen()
			if err != nil {
				// closing channel will eventually cancel s,
				// so, just delegate to defe
				s.abort(err)
				return
			}

			select {
			case s.stream <- v:
				continue
			case <-s.Done():
				// cancled, value discarded
				return
			}
		}
	}()

	return s
}

func move[T any, U any](up *state[T], apply func(T) (U, error)) *state[U] {
	return flatten(up, func(v T) (*state[U], error) {
		u, err := apply(v)
		if err != nil {
			return nil, err
		}

		return once(u), nil
	}, true)
}

func flatten[T any, U any](up *state[T], apply func(T) (*state[U], error), run_sync bool) *state[U] {
	ctx, cancle := context.WithCancel(up)
	s := &state[U]{
		Locker: &sync.Mutex{},

		error:  nil,
		stream: make(chan U),

		Context:    ctx,
		CancelFunc: cancle,
	}

	// semantic:
	// 1. states shared the same root context
	// 2. an context cancelation should back propagate
	// 3. an canceld context imply channel closed and vice versa
	// 4. channel should close at defer and after cancelation called
	go func() {
		// in defer when context is canceled
		defer func() {
			up.abort(s.Err())
			close(s.stream)
		}()

		for {
			select {
			case v, more := <-up.stream:
				if !more {
					s.abort(up.Err())
					return
				}

				// transform
				u, err := apply(v)
				if err != nil {
					s.abort(err)
					return
				}

				// pipe
				if run_sync {
					joinIntermidiate(up, u, s)
				} else {
					go joinIntermidiate(up, u, s)
				}

				// next run
				continue
			case <-s.Done():
				// s maybe aborted
				return
			}
		}
	}()
	return s
}

func joinIntermidiate[T any, U any](up *state[U], intermediate *state[T], down *state[T]) {
	// down pipeing may had clsoed, thus broken sending
	defer func() {
		if why := recover(); why != nil {
			intermediate.abort(fmt.Errorf("down state borken:%v nested:%v", why, down.Err()))
			return
		}

		intermediate.abort(down.Err())
	}()

	for {
		select {
		case <-intermediate.Done():
			if intermediate.Err() != nil {
				down.abort(intermediate.Err())
			}
			return
		case v, more := <-intermediate.stream:
			if !more {
				// intermedia is done.
				intermediate.abort(nil)
				return
			}

			select {
			case down.stream <- v:
				// pipe
				// for abnormal case,see defer
				continue
			case <-down.Done():
				// defer suspend
				return
			}
		case <-down.Done():
			// up down are sharing same root context,
			// check for either one
			return
		}
	}
}

func reduce[L any, R any, O any](left *state[L], right *state[R], apply func(L, R) (O, error)) *state[O] {
	ctx, cancle := context.WithCancel(context.TODO())

	s := &state[O]{
		Locker: &sync.Mutex{},

		error:  nil,
		stream: make(chan O),

		Context:    ctx,
		CancelFunc: cancle,
	}

	// bidirectional propagate
	go func() {
		defer func() {
			left.abort(s.Err())
			right.abort(s.Err())
		}()

		select {
		case <-left.Done():
			s.abort(left.Err())
		case <-right.Done():
			s.abort(right.Err())
		case <-s.Done():
			return
		}
	}()

	go func() {
		defer close(s.stream)

		for {
			// first poll left
			lv, lmore := <-left.stream
			if !lmore {
				s.abort(left.Err())
				return
			}

			rv, rmore := <-right.stream
			if !rmore {
				s.abort(right.Err())
				return
			}

			o, err := apply(lv, rv)
			if err != nil {
				s.abort(err)
				return
			}

			select {
			case s.stream <- o:
				continue
			case <-s.Done():
				// cancled by either left or right
				return
			}
		}
	}()

	return s
}
