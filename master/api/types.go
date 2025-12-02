package api

type Response struct {
	Success bool `json:"success"`
	Message string `json:"message"`
}

func FailedResponse(msg string) Response {
	return Response{Success: false, Message: msg}
}

type ProblemResult struct {
	problemID string
	population []Individual
}

type Individual struct {
	fitness float32
	genotype string
}
