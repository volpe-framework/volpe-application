package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
)

func main() {
	target := os.Getenv("VOLPE_ENDPOINT")
	if target == "" {
		target = "http://localhost:8000"
	}

	optionsMsg := `1. n - register Problem
2. s - start Problem
3. r - stream Problem Results
4. k - abort Problem
Any other option to exit`

	done := false
	
	for !done {
		fmt.Print(optionsMsg + "\n")
		option := ""
		fmt.Scan(&option)
		switch option {
		case "n":
			// registerProblem(client)
			fmt.Println("Please use insomnia for this")
		case "s":
			fmt.Println("Please use insomnia for this")
		case "r":
			streamResults(target)
		default: done = true
		}
	}
}

func streamResults(endpoint string) {
	problemID := ""
	fmt.Print("Enter the problemID: ")
	fmt.Scan(&problemID)

	resp, err := http.Get(endpoint + "/problems/"+problemID+"/results")
	if err != nil {
		panic(err)
	}

	rd := bufio.NewReader(resp.Body)

	defer resp.Body.Close()

	bufLines := make([]string, 0, 10)

	for {
		nextLine, err := rd.ReadString('\n')
		if err != nil {
			fmt.Println(bufLines)
			break
		}
		if len(nextLine) == 1 {
			fmt.Println(bufLines)
			clear(bufLines)
		}
	}
}
