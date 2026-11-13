package notify

type Notifier interface{ Notify(msg string) error }

type Email struct{}

func (Email) Notify(msg string) error { return nil }

type SMS struct{}

func (SMS) Notify(msg string) error { return nil }

type Service struct{ N Notifier }

// Send: two concrete Notifiers reach this site.
func (s Service) Send(msg string) error { return s.N.Notify(msg) }

// Audit has no implementation anywhere in the module.
type Audit interface{ Record(msg string) }

func Log(a Audit, msg string) { a.Record(msg) }

// Apply is never called, so nothing is known about f.
func Apply(f func(string) string, s string) string { return f(s) }

func Each[T any](xs []T, f func(T)) {
	for _, x := range xs {
		f(x)
	}
}

func Count(yield func(int) bool) { yield(1) }

func Double(v int) int { return v * 2 }
