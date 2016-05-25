package relay

type Client struct {
	connections []*Connection
}

func NewClient() *Client {
	return new(Client)
}

func ConnectTo() error {
	return nil
}

func Listen() error {
	for {
		select {}
	}
}
