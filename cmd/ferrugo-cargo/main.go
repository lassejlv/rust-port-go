package main

import (
	"github.com/lassejlv/rust-port-go/internal/cargo"
	"os"
)

func main() { os.Exit(cargo.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
