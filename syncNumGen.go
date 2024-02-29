package main

import (
	"context"
	"sync"
)

type sequenced interface {
	getSequence() int
}

type Num struct {
	int
}

func (n Num) getSequence() int {
	return n.int
}

func EvenNumbersGen(ctx context.Context, numbers ...Num) <-chan Num {
	out := make(chan Num)
	go func() {
		defer close(out)
		for _, num := range numbers {
			select {
			case <-ctx.Done():
				return
			default:
				if num.int%2 == 0 {
					out <- num
				}
			}
		}
	}()
	return out
}

func OddNumbersGen(ctx context.Context, numbers ...Num) <-chan Num {
	out := make(chan Num)
	go func() {
		defer close(out)
		for _, num := range numbers {
			select {
			case <-ctx.Done():
				return
			default:
				if num.int%2 != 0 {
					out <- num
				}
			}
		}
	}()
	return out
}

type fanInRecord[T sequenced] struct {
	index int
	data  T
	pause chan struct{}
}

func inTemp[T sequenced](ctx context.Context, channels ...<-chan T) <-chan fanInRecord[T] {
	fanInCh := make(chan fanInRecord[T])
	wg := sync.WaitGroup{}
	for i := range channels {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			pauseCh := make(chan struct{})
			for {
				select {
				case data, ok := <-channels[index]:
					if !ok {
						return
					}
					fanInCh <- fanInRecord[T]{
						index: index,
						data:  data,
						pause: pauseCh,
					}
				case <-ctx.Done():
					return
				}
				select {
				case <-pauseCh:
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(fanInCh)
	}()
	return fanInCh
}

func processTempCh[T sequenced](ctx context.Context, inputChannelsNum int, fanInCh <-chan fanInRecord[T]) <-chan T {
	outputCh := make(chan T)
	go func() {
		defer close(outputCh)
		expected := 0
		queuedData := make([]*fanInRecord[T], inputChannelsNum)
		for in := range fanInCh {
			if in.data.getSequence() == expected {
				select {
				case outputCh <- in.data:
					in.pause <- struct{}{}
					expected++
				case <-ctx.Done():
					return
				}
			} else {
				queuedData[in.index] = &in
			}
			for queuedData[expected%inputChannelsNum] != nil {
				select {
				case outputCh <- queuedData[expected%inputChannelsNum].data:
					queuedData[expected%inputChannelsNum].pause <- struct{}{}
					queuedData[expected%inputChannelsNum] = nil
					expected++
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return outputCh
}
