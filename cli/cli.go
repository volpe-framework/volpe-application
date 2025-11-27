package main

import (
	"context"
	"fmt"
	"os"
	"volpe-framework/comms/api"
	"volpe-framework/comms/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	target := os.Getenv("VOLPE_ENDPOINT")
	if target == "" {
		target = "localhost:8000"
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	client := api.NewVolpeAPIClient(conn)

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
			registerProblem(client)
		case "s":
			startProblem(client)
		case "r":
			streamResults(client)

		default: done = true
		}
	}
}

func registerProblem(client api.VolpeAPIClient) {
	fname := ""
	problemID := ""

	fmt.Print("Enter the file name: ")
	fmt.Scan(&fname)

	fmt.Print("Enter the problem ID: ")
	fmt.Scan(&problemID)

	stream, err := client.RegisterProblem(context.Background())
	if err != nil {
		panic(err)
	}

	stat, err := os.Stat(fname)
	if err != nil {
		panic(err)
	}
	fileSize := int(stat.Size())

	err = stream.Send(&common.ImageStreamObject{
		Data: &common.ImageStreamObject_Details{
			Details: &common.ImageDetails{
				ProblemID: problemID,
				ImageSizeBytes: int32(fileSize),
			},
		},
	})
	if err != nil {
		panic(err)
	}

	file, _ := os.Open(fname)
	defer file.Close()

	done := 0
	buf := make([]byte, 512)
	for done < fileSize {
		cur, _ := file.Read(buf)
		stream.Send(&common.ImageStreamObject{
			Data: &common.ImageStreamObject_Chunk{
				Chunk: &common.ImageChunk{
					Data: buf,
				},
			},
		})
		done += cur
	}
	result, err := stream.CloseAndRecv()
	if err != nil {
		panic(err)
	}
	if !result.Success {
		fmt.Print("error: " + result.GetErrorMessage())
	}
}

func startProblem(client api.VolpeAPIClient) {
	problemID := ""
	fmt.Print("Enter the problemID: ")
	fmt.Scan(&problemID)

	result, err := client.StartProblem(context.Background(), &api.StartProblemRequest{ProblemID: problemID})
	if err != nil {
		panic(err)
	}
	if !result.Success {
		fmt.Print("error: " + result.GetErrorMessage())
	}
}

func streamResults(client api.VolpeAPIClient) {
	problemID := ""
	fmt.Print("Enter the problemID: ")
	fmt.Scan(&problemID)

	stream, err := client.StreamResults(context.Background(), &api.StreamResultsRequest{ProblemID: problemID})
	if err != nil {
		panic(err)
	}
	for {
		msg, err := stream.Recv()
		if err != nil {
			fmt.Println("exiting with error ", err.Error())
			break
		}
		best := msg.GetBestResults()
		for member := range best.Members {
			fmt.Print(best.Members[member].GetFitness(), " ")
		}
		fmt.Print("\n")
	}
}
