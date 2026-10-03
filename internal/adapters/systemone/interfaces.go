package systemone

//go:generate go tool mockgen -destination=interfaces_mock_test.go -package=systemone net/http RoundTripper
//go:generate go tool mockgen -destination=context_mock_test.go -package=systemone context Context
