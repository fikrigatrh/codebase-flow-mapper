package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
)

type Item struct{ ID string }

func load(id string) (Item, error) { return Item{ID: id}, nil }
func save(it Item) error           { return nil }
func seq(yield func(int) bool)     { _ = yield(1) }

func Process(ctx context.Context, ids []string, ch chan int) (int, error) {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	defer close(ch)
	n := len(ids)
	if n == 0 {
		return 0, errors.New("no ids")
	} else if n > 100 {
		panic("too many: " + strconv.Itoa(n))
	} else {
		n++
	}
	for _, id := range ids {
		it, err := load(id)
		if err != nil {
			continue
		}
		switch it.ID {
		case "skip":
		case "a", "b":
			_ = save(it)
		default:
			fmt.Println(it.ID)
		}
	}
	for i := 0; i < 3; i++ {
		n += i
	}
	for v := range seq {
		n += int64ToInt(int64(v))
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case v := <-ch:
		n += v
	}
	go func() { _ = save(Item{}) }()
	var x any = n
	switch t := x.(type) {
	case int:
		if t == 1 || t == 2 || t == 3 || t == 4 || t == 5 || t == 6 || t == 7 || t == 8 || t == 9 || t == 10 || t == 11 || t == 12 || t == 13 {
			return t, nil
		}
	}
	return n, save(Item{ID: fmt.Sprint(n)})
}

func int64ToInt(v int64) int { return int(v) }

func main() { _, _ = Process(context.Background(), os.Args, make(chan int)) }
