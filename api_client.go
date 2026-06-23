package lucima

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

var Cli = &ApiClient{ApiAddr: "localhost:8989"}

var httpClient = &http.Client{Transport: &http.Transport{}}

type ApiClient struct {
	ApiAddr string
}

func (client *ApiClient) ServerStarted(name, port string) error {
	return client.serverPort("POST", name, port)
}

func (client *ApiClient) ServerStopped(name, port string) error {
	return client.serverPort("DELETE", name, port)
}

func (client *ApiClient) serverPort(method, name, port string) error {
	url := fmt.Sprintf("http://%v/serverPort?name=%v&port=%v", client.ApiAddr, name, port)
	req, _ := http.NewRequest(method, url, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errorFromResponse(resp)
		log.Println(err)
		return err
	}
	return nil
}

func (client *ApiClient) ClientConnected(name, port string) error {
	return client.clientPort("POST", name, port)
}

func (client *ApiClient) ClientDisconnected(port string) error {
	return client.clientPort("DELETE", "", port)
}

func (client *ApiClient) clientPort(method, name, port string) error {
	url := fmt.Sprintf("http://%v/clientPort?name=%v&port=%v", client.ApiAddr, name, port)
	req, _ := http.NewRequest(method, url, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errorFromResponse(resp)
		log.Println(err)
		return err
	}
	return nil
}

func errorFromResponse(resp *http.Response) error {
	body := make([]byte, resp.ContentLength)
	io.ReadFull(resp.Body, body)
	return errors.New(string(body))
}
