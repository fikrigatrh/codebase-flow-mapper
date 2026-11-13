package main

import (
	"os"

	"example.com/dynamic/internal/notify"
)

func main() {
	var n notify.Notifier = notify.Email{}
	if len(os.Args) > 1 {
		n = notify.SMS{}
	}
	s := notify.Service{N: n}
	send := s.Send
	_ = send("hi")
	notify.Each([]string{"a"}, func(m string) { _ = s.Send(m) })
	notify.Log(nil, "x")
	for v := range notify.Count {
		_ = notify.Double(v)
	}
}
