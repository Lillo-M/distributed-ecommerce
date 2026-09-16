package main

import "eCommerce/pkg/helpers"

func main() {
	tui, err := CreateTUI()
	helpers.FailOnError(err, "Failed to create TUI")
	defer tui.DestroyTUI()

	var forever chan int

	tui.TestSend("Hello, World!")
	<-forever
}
