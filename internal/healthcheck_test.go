package internal

import (
	"reflect"
	"testing"
)

func TestCumulativeSlidingWindow_Add(t *testing.T) {
	windowSize := CumulativeSlidingWindowSize

	type args struct {
		failures []bool
	}
	cases := []struct {
		name string
		args args
		want *CumulativeSlidingWindow
	}{
		{
			name: "initial should be all true",
			args: args{
				failures: []bool{},
			},
			want: &CumulativeSlidingWindow{
				size:     windowSize,
				window:   []bool{true, true, true, true, true},
				failures: windowSize,
				cursor:   0,
			},
		},
		{
			name: "no failures",
			args: args{
				failures: []bool{false, false, false, false, false, false},
			},
			want: &CumulativeSlidingWindow{
				size:     windowSize,
				window:   []bool{false, false, false, false, false},
				failures: 0,
				cursor:   1,
			},
		},
		{
			name: "two failure at same position",
			args: args{
				failures: []bool{true, false, false, false, false, true},
			},
			want: &CumulativeSlidingWindow{
				size:     windowSize,
				window:   []bool{true, false, false, false, false},
				failures: 1,
				cursor:   1,
			},
		},
		{
			name: "failure change to no failure",
			args: args{
				failures: []bool{true, false, false, false, false, false},
			},
			want: &CumulativeSlidingWindow{
				size:     windowSize,
				window:   []bool{false, false, false, false, false},
				failures: 0,
				cursor:   1,
			},
		},
		{
			name: "no failure change to failure",
			args: args{
				failures: []bool{false, false, false, false, false, true},
			},
			want: &CumulativeSlidingWindow{
				size:     windowSize,
				window:   []bool{true, false, false, false, false},
				failures: 1,
				cursor:   1,
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewCumulativeSlidingWindow(windowSize)
			for _, failure := range c.args.failures {
				w.Add(failure)
			}
			if got := w; !reflect.DeepEqual(got, c.want) {
				t.Errorf("CumulativeSlidingWindow.Add() = %v, want %v", *got, *c.want)
			}
		})
	}
}
