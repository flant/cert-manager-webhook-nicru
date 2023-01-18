package main

func NewNicruClient(token string) *NicruClient {
	return &NicruClient{
		token: token,
	}
}
