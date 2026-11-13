package app

type App struct{ name string }

// New gained a parameter after wire_gen.go was last generated.
func New(name string) *App { return &App{name: name} }
