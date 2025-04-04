package main

func Handler(req Request) (Response, error) {
	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: "Hello from handler!",
	}, nil
}
