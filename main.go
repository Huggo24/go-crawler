package main

import (
	"fmt"
	"os"
	"strconv"
)

const errorMessage string = "Usage: go run . <url> <depth> <timeout(ms)> [workers]"

func main() {
	//Первый аргумент - адрес исполняемого файла
	if len(os.Args) < 4 || len(os.Args) > 5 {
		fmt.Println(errorMessage)
		return
	}

	link := os.Args[1]

	depth, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println("Depth is NaN")
		fmt.Println(errorMessage)
		return
	}

	timeout, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Println("Timeout is NaN")
		fmt.Println(errorMessage)
		return
	}

	workerPool := 10
	if len(os.Args) > 4 {
		workerPool, err = strconv.Atoi(os.Args[4])
		if err != nil {
			fmt.Println("Thread number is NaN")
			fmt.Println(errorMessage)
			return
		}
	}

	StartCrawler(link, depth, timeout, workerPool)
}
