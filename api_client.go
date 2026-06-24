package lucima

import (
	"encoding/json"
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

func (client *ApiClient) DialState(clientName, serverPort string) (state ConnState, err error) {
	url := fmt.Sprintf("http://%v/dialState?clientName=%v&serverPort=%v", client.ApiAddr, clientName, serverPort)
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		err = errorFromResponse(resp)
		log.Println(err)
		return
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}
	err = json.Unmarshal(data, &state)
	if err != nil {
		log.Println(err)
	}
	return
}

func (client *ApiClient) ConnState(clientPort, serverPort string, oldState *ConnState) (state *ConnState, err error) {
	url := fmt.Sprintf("http://%v/connState?clientPort=%v&serverPort=%v", client.ApiAddr, clientPort, serverPort)
	req, _ := http.NewRequest("GET", url, nil)
	if oldState != nil {
		jsonBytes, _ := json.Marshal(oldState)
		req.Header.Add("If-None-Match", string(jsonBytes))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode == 304 {
		state = oldState
		return
	}
	if resp.StatusCode != 200 {
		err = errorFromResponse(resp)
		log.Println(err)
		return
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}
	state = new(ConnState)
	err = json.Unmarshal(data, state)
	if err != nil {
		log.Println(err)
	}
	return
}
