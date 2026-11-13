module example.com/missingdep

go 1.23

require (
	example.com/doesnotexist v1.0.0
	github.com/gofiber/fiber/v3 v3.0.0
)
