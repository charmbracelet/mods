package main

import "sync"

// copied from go1.21, remove and use sync.OnceValue once go 1.21 is widely
// available in package managers.

// OnceValue returns a function that invokes f only once and returns the value
// returned by f. The returned function may be called concurrently.
//
// If f panics, the returned function will panic with the same value on every call.
func OnceValue[T any](f func() T) func() T {
	var once sync.Once
	var valid bool
	var p any
	var result T
	return func() T {
		once.Do(func() {
			defer func() {
				p = recover()
				if !valid {
					panic(p)
				}
			}()
			result = f()
			valid = true
		})
		if !valid {
			panic(p)
		}
		return result
	}
}
