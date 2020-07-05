package main

import "fmt"


func main() {
	b := []int{1,2,3,4,5,6}
	fmt.Println(b)
	b = append(b, b[2])
	fmt.Println(b)
	b[2] = 10
	fmt.Println(b)
}
