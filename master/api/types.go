package api

type Response struct {
	Success bool `json:"success"`
	Message string `json:"message"`
}

func FailedResponse(msg string) Response {
	return Response{Success: false, Message: msg}
}

type ProblemResult struct {
	ProblemID string `json:"problemID"`
	Population []Individual `json:"population"`
}

type Individual struct {
	Fitness float32 `json:"fitness"`
	Genotype string `json:"genotype"`
}
