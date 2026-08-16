package main

import (
	"github.com/antoniomiletta/pengu/pkg/client"
)

func main() {
	pengu := client.NewDB("data/data.db")

	val, _ := pengu.Get("ai")
	println(val)
}
